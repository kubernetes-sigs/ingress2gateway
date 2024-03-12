/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package istio

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	istiov1beta1 "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type converter struct {
	// gw -> namespace -> hosts; stores hosts allowed by each Gateway
	gwAllowedHosts map[types.NamespacedName]map[string]sets.Set[string]
}

func newConverter() converter {
	return converter{
		gwAllowedHosts: make(map[types.NamespacedName]map[string]sets.Set[string]),
	}
}

func (c *converter) convert(storage *storage) (i2gw.GatewayResources, field.ErrorList) {
	var errList field.ErrorList

	gatewayResources := i2gw.GatewayResources{
		Gateways:        make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes:      make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		TLSRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		ReferenceGrants: make(map[types.NamespacedName]gatewayv1alpha2.ReferenceGrant),
	}

	rootPath := field.NewPath(ProviderName)

	for _, istioGateway := range storage.Gateways {
		gw, errors := c.convertGateway(istioGateway, rootPath)
		if len(errors) > 0 {
			errList = append(errList, errors...)
			continue
		}

		gatewayResources.Gateways[types.NamespacedName{
			Namespace: gw.Namespace,
			Name:      gw.Name,
		}] = *gw
	}

	for _, vs := range storage.VirtualServices {
		vsFieldPath := rootPath.Child("VirtualService").Key(types.NamespacedName{
			Namespace: vs.Namespace,
			Name:      vs.Name,
		}.String())

		parentRefs, referenceGrants := c.generateReferences(vs, vsFieldPath)

		httpRoutes, errors := c.convertVsHTTPRoutes(vs.ObjectMeta, vs.Spec.GetHttp(), vs.Spec.GetHosts(), vsFieldPath)
		if len(errors) > 0 {
			errList = append(errList, errors...)
		} else {
			for _, httpRoute := range httpRoutes {
				httpRoute.Spec.ParentRefs = parentRefs
				gatewayResources.HTTPRoutes[types.NamespacedName{
					Namespace: httpRoute.Namespace,
					Name:      httpRoute.Name,
				}] = *httpRoute
			}
		}

		for _, tlsRoute := range c.convertVsTLSRoutes(vs.ObjectMeta, vs.Spec.GetTls(), vsFieldPath) {
			tlsRoute.Spec.ParentRefs = parentRefs
			gatewayResources.TLSRoutes[types.NamespacedName{
				Namespace: tlsRoute.Namespace,
				Name:      tlsRoute.Name,
			}] = *tlsRoute
		}

		for _, tcpRoute := range c.convertVsTCPRoutes(vs.ObjectMeta, vs.Spec.GetTcp(), vsFieldPath) {
			tcpRoute.Spec.ParentRefs = parentRefs
			gatewayResources.TCPRoutes[types.NamespacedName{
				Namespace: tcpRoute.Namespace,
				Name:      tcpRoute.Name,
			}] = *tcpRoute
		}

		for _, rg := range referenceGrants {
			gatewayResources.ReferenceGrants[types.NamespacedName{
				Namespace: rg.Namespace,
				Name:      rg.Name,
			}] = *rg
		}
	}

	return gatewayResources, nil
}

func (c *converter) convertGateway(gw *istioclientv1beta1.Gateway, fieldPath *field.Path) (*gatewayv1.Gateway, field.ErrorList) {
	var errList field.ErrorList
	apiVersion, kind := common.GatewayGVK.ToAPIVersionAndKind()
	gwPath := fieldPath.Child("Gateway").Key(gw.Name)

	var listeners []gatewayv1.Listener

	// namespace -> hosts
	gwAllowedHosts := make(map[string]sets.Set[string])

	for i, server := range gw.Spec.GetServers() {
		serverName := fmt.Sprintf("%v", i)
		if server.GetName() != "" {
			serverName = server.GetName()
		}
		serverFieldPath := gwPath.Child("Server").Key(serverName)

		serverPort := server.GetPort()
		if serverPort == nil {
			klog.Error(field.Invalid(serverFieldPath, nil, "port is nil"))
			continue
		}

		portFieldPath := serverFieldPath.Child("Port")

		if serverPort.GetName() != "" {
			klog.Infof("ignoring field: %v", portFieldPath.Child("Name"))
		}

		var protocol gatewayv1.ProtocolType
		switch serverPortProtocol := serverPort.GetProtocol(); serverPortProtocol {
		case "HTTP", "HTTPS", "TCP", "TLS":
			protocol = gatewayv1.ProtocolType(serverPortProtocol)
		case "HTTP2", "GRPC":
			if server.GetTls() != nil {
				protocol = gatewayv1.HTTPSProtocolType
			} else {
				protocol = gatewayv1.HTTPProtocolType
			}
		case "MONGO":
			protocol = gatewayv1.TCPProtocolType
		default:
			errList = append(errList, field.Invalid(portFieldPath.Child("Protocol"), serverPortProtocol, "unknown istio server protocol"))
			continue
		}

		var tlsMode gatewayv1.TLSModeType
		if serverTLS := server.GetTls(); serverTLS != nil {
			tlsFieldPath := serverFieldPath.Child("TLS")

			switch serverTLSMode := serverTLS.GetMode(); serverTLSMode {
			case istiov1beta1.ServerTLSSettings_PASSTHROUGH, istiov1beta1.ServerTLSSettings_AUTO_PASSTHROUGH:
				tlsMode = gatewayv1.TLSModePassthrough
			case istiov1beta1.ServerTLSSettings_SIMPLE, istiov1beta1.ServerTLSSettings_MUTUAL:
				tlsMode = gatewayv1.TLSModeTerminate
			case istiov1beta1.ServerTLSSettings_ISTIO_MUTUAL, istiov1beta1.ServerTLSSettings_OPTIONAL_MUTUAL:
				klog.Warningf("the istio server is ignored as there's no direct translation for this TLS istio protocol: %v", tlsFieldPath.Child("Mode").Key(serverTLSMode.String()))
				continue
			default:
				errList = append(errList, field.Invalid(tlsFieldPath.Child("Mode"), serverTLSMode, "unknown istio server tls mode"))
			}

			if serverTLS.GetHttpsRedirect() {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("HttpsRedirect"))
			}
			if serverTLS.GetServerCertificate() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("ServerCertificate"))
			}
			if serverTLS.GetPrivateKey() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("PrivateKey"))
			}
			if serverTLS.GetCaCertificates() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("CaCertificates"))
			}
			if len(serverTLS.GetSubjectAltNames()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("SubjectAltNames"))
			}
			if serverTLS.GetCredentialName() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("CredentialName"))
			}
			if len(serverTLS.GetVerifyCertificateSpki()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("VerifyCertificateSpki"))
			}
			if len(serverTLS.GetVerifyCertificateHash()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("VerifyCertificateHash"))
			}
			if serverTLS.GetMinProtocolVersion() != 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("MinProtocolVersion"))
			}
			if serverTLS.GetMaxProtocolVersion() != 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("MaxProtocolVersion"))
			}
			if len(serverTLS.GetCipherSuites()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("CipherSuites"))
			}
		}

		if server.GetBind() != "" {
			klog.Infof("ignoring field: %v", serverFieldPath.Child("Bind").Key(server.GetBind()))
		}

		for _, host := range server.GetHosts() {
			gwListener := gatewayv1.Listener{
				Port:     gatewayv1.PortNumber(server.GetPort().GetNumber()),
				Protocol: protocol,
			}
			if tlsMode != "" {
				gwListener.TLS = &gatewayv1.GatewayTLSConfig{
					Mode: &tlsMode,
				}
			}

			namespace, dnsName, ok := strings.Cut(host, "/")
			if !ok {
				// The default, if no `namespace/` is specified, is `*/`, that is, select services from any namespace.
				namespace, dnsName = "*", host
			}

			// if dnsName == "*", then gwListener is empty which matches all hostnames for the listener
			if dnsName != "*" {
				gwListener.Hostname = common.PtrTo[gatewayv1.Hostname](gatewayv1.Hostname(dnsName))
			}

			if _, ok := gwAllowedHosts[namespace]; !ok {
				gwAllowedHosts[namespace] = sets.New[string]()
			}
			gwAllowedHosts[namespace].Insert(dnsName)

			gwListenerName := strings.ToLower(fmt.Sprintf("%v-protocol-%v-ns-%v", protocol, namespace, dnsName))
			if namespace == "." {
				gwListenerName = strings.ToLower(fmt.Sprintf("%v-protocol-dot-ns-%v", protocol, dnsName))
			}
			gwListenerName = strings.Replace(gwListenerName, "*", "wildcard", -1)

			// listener name should match RFC 1123 subdomain requirement: lowercase alphanumeric characters, '-' or '.', and must start and end with a lowercase alphanumeric character
			gwListener.Name = gatewayv1.SectionName(gwListenerName)

			listeners = append(listeners, gwListener)
		}
	}

	if len(errList) > 0 {
		return nil, errList
	}

	c.gwAllowedHosts[types.NamespacedName{
		Namespace: gw.Namespace,
		Name:      gw.Name,
	}] = gwAllowedHosts

	return &gatewayv1.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       gw.Namespace,
			Name:            gw.Name,
			Labels:          gw.Labels,
			Annotations:     gw.Annotations,
			OwnerReferences: gw.OwnerReferences,
			Finalizers:      gw.Finalizers,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: K8SGatewayClassName,
			Listeners:        listeners,
		},
	}, nil
}

func (c *converter) convertVsHTTPRoutes(virtualService metav1.ObjectMeta, istioHTTPRoutes []*istiov1beta1.HTTPRoute, allowedHostnames []string, fieldPath *field.Path) ([]*gatewayv1.HTTPRoute, field.ErrorList) {
	var errList field.ErrorList
	var resHTTPRoutes []*gatewayv1.HTTPRoute

	for i, httpRoute := range istioHTTPRoutes {
		httpRouteFieldName := fmt.Sprintf("%v", i)
		if httpRoute.GetName() != "" {
			httpRouteFieldName = httpRoute.GetName()
		}
		httpRouteFieldPath := fieldPath.Child("Http").Key(httpRouteFieldName)

		var gwHTTPRouteMatches []gatewayv1.HTTPRouteMatch
		var gwHTTPRouteFilters []gatewayv1.HTTPRouteFilter

		for j, match := range httpRoute.GetMatch() {
			httpMatchFieldName := fmt.Sprintf("%v", j)
			if match.GetName() != "" {
				httpMatchFieldName = match.GetName()
			}
			httpMatchFieldPath := httpRouteFieldPath.Child("HTTPMatchRequest").Key(httpMatchFieldName)

			if match.GetScheme() != nil {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("Scheme").Key(match.GetScheme().String()))
			}
			if match.GetAuthority() != nil {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("Authority").Key(match.GetAuthority().String()))
			}
			if match.GetPort() != 0 {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("Port").Key(fmt.Sprintf("%v", match.GetPort())))
			}
			if len(match.GetSourceLabels()) > 0 {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("SourceLabels"))
			}
			if match.GetIgnoreUriCase() {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("IgnoreUriCase"))
			}
			if len(match.GetWithoutHeaders()) > 0 {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("WithoutHeaders"))
			}
			if match.GetSourceNamespace() != "" {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("SourceNamespace"))
			}
			if match.GetStatPrefix() != "" {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("StatPrefix"))
			}
			if len(match.GetGateways()) > 0 {
				klog.Infof("ignoring field: %v", httpMatchFieldPath.Child("Gateways"))
			}

			gwHTTPRouteMatch := gatewayv1.HTTPRouteMatch{}

			if matchURI := match.GetUri(); matchURI != nil {
				var (
					matchType gatewayv1.PathMatchType
					value     string
				)

				switch matchURI.GetMatchType().(type) {
				case *istiov1beta1.StringMatch_Exact:
					matchType = gatewayv1.PathMatchExact
					value = matchURI.GetExact()
				case *istiov1beta1.StringMatch_Prefix:
					matchType = gatewayv1.PathMatchPathPrefix
					value = matchURI.GetPrefix()
				case *istiov1beta1.StringMatch_Regex:
					matchType = gatewayv1.PathMatchRegularExpression
					value = matchURI.GetRegex()
				default:
					klog.Error(field.Invalid(httpMatchFieldPath.Child("Uri"), matchURI, "unsupported Uri match type %v"))
				}

				if matchType != "" {
					gwHTTPRouteMatch.Path = &gatewayv1.HTTPPathMatch{
						Type:  &matchType,
						Value: &value,
					}
				}
			}

			for header, headerMatch := range match.GetHeaders() {
				var (
					matchType gatewayv1.HeaderMatchType
					value     string
				)

				switch headerMatch.GetMatchType().(type) {
				case *istiov1beta1.StringMatch_Exact:
					matchType = gatewayv1.HeaderMatchExact
					value = headerMatch.GetExact()
				case *istiov1beta1.StringMatch_Regex:
					matchType = gatewayv1.HeaderMatchRegularExpression
					value = headerMatch.GetRegex()
				default:
					klog.Error(field.Invalid(httpMatchFieldPath.Child("Headers"), headerMatch, "unsupported Headers match type"))
				}

				if matchType != "" {
					gwHTTPRouteMatch.Headers = append(gwHTTPRouteMatch.Headers, gatewayv1.HTTPHeaderMatch{
						Type:  &matchType,
						Name:  gatewayv1.HTTPHeaderName(header),
						Value: value,
					})
				}
			}

			for query, queryMatch := range match.GetQueryParams() {
				var (
					matchType gatewayv1.QueryParamMatchType
					value     string
				)

				switch queryMatch.GetMatchType().(type) {
				case *istiov1beta1.StringMatch_Exact:
					matchType = gatewayv1.QueryParamMatchExact
					value = queryMatch.GetExact()
				case *istiov1beta1.StringMatch_Regex:
					matchType = gatewayv1.QueryParamMatchRegularExpression
					value = queryMatch.GetRegex()
				default:
					klog.Error(field.Invalid(httpMatchFieldPath.Child("QueryParams"), queryMatch, "unsupported QueryParams match type"))
				}

				if matchType != "" {
					gwHTTPRouteMatch.QueryParams = append(gwHTTPRouteMatch.QueryParams, gatewayv1.HTTPQueryParamMatch{
						Type:  &matchType,
						Name:  gatewayv1.HTTPHeaderName(query),
						Value: value,
					})
				}
			}

			if matchMethod := match.GetMethod(); matchMethod != nil {
				switch matchMethod.GetMatchType().(type) {
				case *istiov1beta1.StringMatch_Exact:
					gwHTTPRouteMatch.Method = common.PtrTo[gatewayv1.HTTPMethod](gatewayv1.HTTPMethod(matchMethod.GetExact()))
				default:
					klog.Error(field.Invalid(httpMatchFieldPath.Child("Method"), matchMethod, "unsupported Method match type"))
				}
			}

			gwHTTPRouteMatches = append(gwHTTPRouteMatches, gwHTTPRouteMatch)
		}

		var backendRefs []gatewayv1.HTTPBackendRef
		for j, routeDestination := range httpRoute.GetRoute() {
			routeDestinationFieldPath := httpRouteFieldPath.Child("HTTPRouteDestination").Index(j)

			if routeDestination.GetHeaders() != nil {
				klog.Infof("ignoring field: %v", routeDestinationFieldPath.Child("Headers"))
			}

			backendObjRef := destination2backendObjRef(routeDestination.GetDestination(), virtualService.Namespace, routeDestinationFieldPath)
			if backendObjRef != nil {
				backendRefs = append(backendRefs, gatewayv1.HTTPBackendRef{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: *backendObjRef,
						Weight:                 &routeDestination.Weight,
					},
				})
			}
		}

		if routeRedirect := httpRoute.GetRedirect(); routeRedirect != nil {
			redirectFieldPath := httpRouteFieldPath.Child("HTTPRedirect")

			if routeRedirect.GetAuthority() != "" {
				klog.Infof("ignoring field: %v", redirectFieldPath.Child("Authority"))
			}
			if _, ok := routeRedirect.GetRedirectPort().(*istiov1beta1.HTTPRedirect_DerivePort); ok {
				klog.Infof("ignoring field: %v", redirectFieldPath.Child("DerivePort"))
			}

			redirectCode := 301
			if routeRedirect.GetRedirectCode() > 0 {
				redirectCode = int(routeRedirect.GetRedirectCode())
			}

			var redirectPath *gatewayv1.HTTPPathModifier

			if routeRedirectURI := routeRedirect.GetUri(); routeRedirectURI != "" {
				redirectPath = &gatewayv1.HTTPPathModifier{
					Type:            gatewayv1.FullPathHTTPPathModifier,
					ReplaceFullPath: &routeRedirectURI,
				}
			}

			redirectFilter := gatewayv1.HTTPRequestRedirectFilter{
				StatusCode: &redirectCode,
				Path:       redirectPath,
			}

			if routeRedirectScheme := routeRedirect.GetScheme(); routeRedirectScheme != "" {
				redirectFilter.Scheme = &routeRedirectScheme
			}

			if routeRedirect.GetPort() > 0 {
				redirectPort := gatewayv1.PortNumber(routeRedirect.GetPort())
				redirectFilter.Port = &redirectPort
			}

			gwHTTPRouteFilters = append(gwHTTPRouteFilters, gatewayv1.HTTPRouteFilter{
				Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &redirectFilter,
			})
		}

		if httpRoute.GetDirectResponse() != nil {
			klog.Infof("ignoring field: %v", httpRouteFieldPath.Child("DirectResponse"))
		}
		if httpRoute.GetDelegate() != nil {
			klog.Infof("ignoring field: %v", httpRouteFieldPath.Child("Delegate"))
		}
		if httpRoute.GetRetries() != nil {
			klog.Infof("ignoring field: %v", httpRouteFieldPath.Child("Retries"))
		}
		if httpRoute.GetFault() != nil {
			klog.Infof("ignoring field: %v", httpRouteFieldPath.Child("Fault"))
		}
		if httpRoute.GetCorsPolicy() != nil {
			klog.Infof("ignoring field: %v", httpRouteFieldPath.Child("CorsPolicy"))
		}

		if httpRoute.GetMirror() != nil && len(httpRoute.GetMirrors()) > 0 {
			errList = append(errList, field.Invalid(httpRouteFieldPath, httpRoute, "HTTP route cannot contain both mirror and mirrors"))
			continue
		}

		if mirror := httpRoute.GetMirror(); mirror != nil {
			routeDestinationFieldPath := httpRouteFieldPath.Child("Mirror")

			backendObjRef := destination2backendObjRef(mirror, virtualService.Namespace, routeDestinationFieldPath)
			if backendObjRef != nil {
				gwHTTPRouteFilters = append(gwHTTPRouteFilters, gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterRequestMirror,
					RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
						BackendRef: *backendObjRef,
					},
				})
			}
		}

		for j, mirror := range httpRoute.GetMirrors() {
			routeDestinationFieldPath := httpRouteFieldPath.Child("Mirrors").Index(j)

			if mirror.GetPercentage() != nil {
				klog.Infof("ignoring field: %v", routeDestinationFieldPath.Child("Percentage"))
			}

			backendObjRef := destination2backendObjRef(mirror.GetDestination(), virtualService.Namespace, routeDestinationFieldPath)
			if backendObjRef != nil {
				gwHTTPRouteFilters = append(gwHTTPRouteFilters, gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterRequestMirror,
					RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
						BackendRef: *backendObjRef,
					},
				})
			}
		}

		var httpRouteTimeouts *gatewayv1.HTTPRouteTimeouts
		if routeTimeout := httpRoute.GetTimeout(); routeTimeout != nil {
			d := gatewayv1.Duration(routeTimeout.AsDuration().String())
			httpRouteTimeouts = &gatewayv1.HTTPRouteTimeouts{
				Request: &d,
			}
		}

		if headers := httpRoute.GetHeaders(); headers != nil {
			if requestHeaders := headers.GetRequest(); requestHeaders != nil {
				gwHTTPRouteFilters = append(gwHTTPRouteFilters, gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Set:    makeHeaderFilter(requestHeaders.GetSet()),
						Add:    makeHeaderFilter(requestHeaders.GetAdd()),
						Remove: requestHeaders.GetRemove(),
					},
				})
			}

			if responseHeaders := headers.GetResponse(); responseHeaders != nil {
				gwHTTPRouteFilters = append(gwHTTPRouteFilters, gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Set:    makeHeaderFilter(responseHeaders.GetSet()),
						Add:    makeHeaderFilter(responseHeaders.GetAdd()),
						Remove: responseHeaders.GetRemove(),
					},
				})
			}
		}

		// set istio hostnames as is, without extra filters. If it's not a fqdn, it would be rejected by K8S API implementation
		hostnames := make([]gatewayv1.Hostname, 0, len(allowedHostnames))
		for _, host := range allowedHostnames {
			hostnames = append(hostnames, gatewayv1.Hostname(host))
		}

		routeName := fmt.Sprintf("%v-idx-%v", virtualService.Name, i)
		if httpRoute.GetName() != "" {
			routeName = fmt.Sprintf("%v-%v", virtualService.Name, httpRoute.GetName())
		}

		createHTTPRouteParams := createHTTPRouteParams{
			objectMeta: metav1.ObjectMeta{
				Namespace:       virtualService.Namespace,
				Name:            routeName,
				Labels:          virtualService.Labels,
				Annotations:     virtualService.Annotations,
				OwnerReferences: virtualService.OwnerReferences,
				Finalizers:      virtualService.Finalizers,
			},
			hostnames:   hostnames,
			matches:     gwHTTPRouteMatches,
			filters:     gwHTTPRouteFilters,
			backendRefs: backendRefs,
			timeouts:    httpRouteTimeouts,
		}

		if httpRoute.GetRewrite() != nil {
			resHTTPRoutes = append(resHTTPRoutes, c.createHTTPRoutesWithRewrite(createHTTPRouteParams, httpRoute.GetRewrite(), httpRouteFieldPath.Child("HTTPRewrite"))...)
			continue
		}

		resHTTPRoutes = append(resHTTPRoutes, c.createHTTPRoute(createHTTPRouteParams))
	}

	if len(errList) > 0 {
		return nil, errList
	}

	return resHTTPRoutes, nil
}

type createHTTPRouteParams struct {
	objectMeta  metav1.ObjectMeta
	hostnames   []gatewayv1.Hostname
	matches     []gatewayv1.HTTPRouteMatch
	filters     []gatewayv1.HTTPRouteFilter
	backendRefs []gatewayv1.HTTPBackendRef
	timeouts    *gatewayv1.HTTPRouteTimeouts
}

func (c *converter) createHTTPRoute(params createHTTPRouteParams) *gatewayv1.HTTPRoute {
	apiVersion, kind := common.HTTPRouteGVK.ToAPIVersionAndKind()

	return &gatewayv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: params.objectMeta,
		Spec: gatewayv1.HTTPRouteSpec{
			Hostnames: params.hostnames,
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches:     params.matches,
					Filters:     params.filters,
					BackendRefs: params.backendRefs,
					Timeouts:    params.timeouts,
				},
			},
		},
	}
}

// createHTTPRoutesWithRewrite generates k8sgw.HTTRoutes taking into consideration "rewrite" option in istio.HTTPRewrite
// In istio, the rewrite logic depends on the match URI parameters:
// 1. for prefix match, istio rewrites matched prefix to the given value.
// 2. for exact match and for regex match, istio rewrites full URI path to the given value.
//
// Also, in K8S Gateway API only 1 HTTPRouteFilterURLRewrite is allowed per HTTPRouteRule
// https://github.com/kubernetes-sigs/gateway-api/blob/0ad0daffe8d47f97a293b2a947bb3b2ee658e967/apis/v1/httproute_types.go#L228
//
// To take this all into consideration, translator aggregates prefix matches vs non-prefix matches
// And generates max 2 HTTPRoutes (one with prefix matches and ReplacePrefixMatch filter and the other if non-prefix matches and ReplaceFullPath filter).
// If any of the match group is empty, the corresponding HTTPRoute won't be generated.
// If all URI matches are empty, there would be HTTPRoute with HTTPRouteFilterURLRewrite of ReplaceFullPath type.
func (c *converter) createHTTPRoutesWithRewrite(params createHTTPRouteParams, rewrite *istiov1beta1.HTTPRewrite, fieldPath *field.Path) []*gatewayv1.HTTPRoute {
	if rewrite == nil {
		return nil
	}

	if rewrite.GetAuthority() != "" {
		klog.Infof("ignoring field: %v", fieldPath.Child("Authority"))
	}
	if rewrite.GetUriRegexRewrite() != nil {
		klog.Infof("ignoring field: %v", fieldPath.Child("UriRegexRewrite"))
	}

	origFilters := params.filters

	var prefixRouteMatches, nonPrefixRouteMatches []gatewayv1.HTTPRouteMatch
	for _, match := range params.matches {
		// if it's a non-path match, then prefixMatch rewrite is generated
		if match.Path == nil {
			prefixRouteMatches = append(prefixRouteMatches, match)
			continue
		}

		// if type == nil, prefixMatch is the default
		if match.Path.Type == nil || *match.Path.Type == gatewayv1.PathMatchPathPrefix {
			prefixRouteMatches = append(prefixRouteMatches, match)
		} else {
			nonPrefixRouteMatches = append(nonPrefixRouteMatches, match)
		}
	}

	var resHTTPRoutes []*gatewayv1.HTTPRoute

	// these matches contain Exact and Regex matches, istio does FullPath rewrite for both
	if len(nonPrefixRouteMatches) > 0 {
		params.filters = append(origFilters, gatewayv1.HTTPRouteFilter{
			Type: gatewayv1.HTTPRouteFilterURLRewrite,
			URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type:            gatewayv1.FullPathHTTPPathModifier,
					ReplaceFullPath: &rewrite.Uri,
				},
			},
		})
		params.matches = nonPrefixRouteMatches

		resHTTPRoutes = append(resHTTPRoutes, c.createHTTPRoute(params))
	}

	// if there are no matches at all istio treats this as a "/" prefix match, same as k8s gateway api expects
	if len(params.matches) == 0 || len(prefixRouteMatches) > 0 {
		params.filters = append(origFilters, gatewayv1.HTTPRouteFilter{
			Type: gatewayv1.HTTPRouteFilterURLRewrite,
			URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type:               gatewayv1.PrefixMatchHTTPPathModifier,
					ReplacePrefixMatch: &rewrite.Uri,
				},
			},
		})
		params.matches = prefixRouteMatches
		params.objectMeta.Name += "-prefix-match"

		resHTTPRoutes = append(resHTTPRoutes, c.createHTTPRoute(params))
	}

	return resHTTPRoutes
}

func (c *converter) convertVsTLSRoutes(virtualService metav1.ObjectMeta, istioTLSRoutes []*istiov1beta1.TLSRoute, fieldPath *field.Path) []*gatewayv1alpha2.TLSRoute {
	var resTLSRoutes []*gatewayv1alpha2.TLSRoute

	for i, route := range istioTLSRoutes {
		tlsRouteFieldPath := fieldPath.Child("Tls").Index(i)

		var backendRefs []gatewayv1.BackendRef
		for _, destination := range route.GetRoute() {
			backendObjRef := destination2backendObjRef(destination.GetDestination(), virtualService.Namespace, tlsRouteFieldPath)
			if backendObjRef != nil {
				backendRefs = append(backendRefs, gatewayv1.BackendRef{
					BackendObjectReference: *backendObjRef,
					Weight:                 &destination.Weight,
				})
			}
		}

		sniHosts := sets.New[gatewayv1.Hostname]()

		for j, match := range route.GetMatch() {
			for _, sniHost := range match.GetSniHosts() {
				sniHosts.Insert(gatewayv1.Hostname(sniHost))
			}

			tlsMatchFieldPath := tlsRouteFieldPath.Child("TLSMatchAttributes").Index(j)

			if len(match.GetDestinationSubnets()) > 0 {
				klog.Infof("ignoring field: %v", tlsMatchFieldPath.Child("DestinationSubnets"))
			}
			if match.GetPort() != 0 {
				klog.Infof("ignoring field: %v", tlsMatchFieldPath.Child("Port"))
			}
			if len(match.GetSourceLabels()) > 0 {
				klog.Infof("ignoring field: %v", tlsMatchFieldPath.Child("SourceLabels"))
			}
			if len(match.GetGateways()) > 0 {
				klog.Infof("ignoring field: %v", tlsMatchFieldPath.Child("Gateways"))
			}
			if match.GetSourceNamespace() != "" {
				klog.Infof("ignoring field: %v", tlsMatchFieldPath.Child("SourceNamespace"))
			}
		}

		apiVersion, kind := common.TLSRouteGVK.ToAPIVersionAndKind()

		routeName := fmt.Sprintf("%v-idx-%v", virtualService.Name, i)

		resTLSRoutes = append(resTLSRoutes, &gatewayv1alpha2.TLSRoute{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       virtualService.Namespace,
				Name:            routeName,
				Labels:          virtualService.Labels,
				Annotations:     virtualService.Annotations,
				OwnerReferences: virtualService.OwnerReferences,
				Finalizers:      virtualService.Finalizers,
			},
			Spec: gatewayv1alpha2.TLSRouteSpec{
				Hostnames: sets.List[gatewayv1.Hostname](sniHosts),
				Rules: []gatewayv1alpha2.TLSRouteRule{
					{
						BackendRefs: backendRefs,
					},
				},
			},
		})
	}

	return resTLSRoutes
}

func (c *converter) convertVsTCPRoutes(virtualService metav1.ObjectMeta, istioTCPRoutes []*istiov1beta1.TCPRoute, fieldPath *field.Path) []*gatewayv1alpha2.TCPRoute {
	var resTCPRoutes []*gatewayv1alpha2.TCPRoute

	for i, route := range istioTCPRoutes {
		tcpRouteFieldPath := fieldPath.Child("Tcp").Index(i)

		var backendRefs []gatewayv1.BackendRef
		for _, destination := range route.GetRoute() {
			backendObjRef := destination2backendObjRef(destination.GetDestination(), virtualService.Namespace, tcpRouteFieldPath)
			if backendObjRef != nil {
				backendRefs = append(backendRefs, gatewayv1.BackendRef{
					BackendObjectReference: *backendObjRef,
					Weight:                 &destination.Weight,
				})
			}
		}

		for j, match := range route.GetMatch() {
			tcpMatchFieldPath := tcpRouteFieldPath.Child("L4MatchAttributes").Index(j)

			if len(match.GetDestinationSubnets()) > 0 {
				klog.Infof("ignoring field: %v", tcpMatchFieldPath.Child("DestinationSubnets"))
			}
			if match.GetPort() != 0 {
				klog.Infof("ignoring field: %v", tcpMatchFieldPath.Child("Port"))
			}
			if match.GetSourceSubnet() != "" {
				klog.Infof("ignoring field: %v", tcpMatchFieldPath.Child("SourceSubnet"))
			}
			if len(match.GetSourceLabels()) > 0 {
				klog.Infof("ignoring field: %v", tcpMatchFieldPath.Child("SourceLabels"))
			}
			if match.GetSourceNamespace() != "" {
				klog.Infof("ignoring field: %v", tcpMatchFieldPath.Child("SourceNamespace"))
			}
			if len(match.GetGateways()) > 0 {
				klog.Infof("ignoring field: %v", tcpMatchFieldPath.Child("Gateways"))
			}
		}

		apiVersion, kind := common.TCPRouteGVK.ToAPIVersionAndKind()

		routeName := fmt.Sprintf("%v-idx-%v", virtualService.Name, i)

		resTCPRoutes = append(resTCPRoutes, &gatewayv1alpha2.TCPRoute{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       virtualService.Namespace,
				Name:            routeName,
				Labels:          virtualService.Labels,
				Annotations:     virtualService.Annotations,
				OwnerReferences: virtualService.OwnerReferences,
				Finalizers:      virtualService.Finalizers,
			},
			Spec: gatewayv1alpha2.TCPRouteSpec{
				Rules: []gatewayv1alpha2.TCPRouteRule{
					{
						BackendRefs: backendRefs,
					},
				},
			},
		})
	}

	return resTCPRoutes
}

func (c *converter) isVirtualServiceAllowedForGateway(gateway types.NamespacedName, vs *istioclientv1beta1.VirtualService, fieldPath *field.Path) bool {
	// by default, if ExportTo is empty it allowes export of the VirtualService to all namespaces
	vsAllowedNamespaces := sets.New("*")
	if len(vs.Spec.GetExportTo()) > 0 {
		vsAllowedNamespaces = sets.New(vs.Spec.GetExportTo()...)
	}

	isAllowedNamespace := vsAllowedNamespaces.HasAny(gateway.Namespace, "*") || (vsAllowedNamespaces.Has(".") && vs.Namespace == gateway.Namespace)
	if !isAllowedNamespace {
		klog.Warningf("gateway from vs.Spec.Gateways %q is not visible in vs.ExportTo %v, parentRefs are not generated for this host, path: %v", gateway.String(), vs.Spec.GetExportTo(), fieldPath)
		return false
	}

	allowedHosts, ok := c.gwAllowedHosts[gateway]
	if !ok {
		klog.Warningf("no info about gateway %v allowed hosts, parentRefs won't be generated to it, path: %v", gateway.String(), fieldPath)
		return false
	}

	for _, host := range vs.Spec.GetHosts() {
		hosts, ok := allowedHosts[vs.Namespace]
		if ok && matchAny(hosts.UnsortedList(), host) {
			return true
		}

		hosts, ok = allowedHosts["."]
		if ok && vs.Namespace == gateway.Namespace && matchAny(hosts.UnsortedList(), host) {
			return true
		}

		hosts, ok = allowedHosts["*"]
		if ok && matchAny(hosts.UnsortedList(), host) {
			return true
		}
	}

	klog.Warningf("no host in vs.Spec.Hosts matched any gateway.allowedHosts, parentRefs are not generated for this VirtualService, path: %v", fieldPath)
	return false
}

// Generate parentRefs and optionally ReferenceGrants for the given VirtualService and all required Gateways
// We consider fields: vs.Spec.Gateways; gateway.Server[i].Hosts
func (c *converter) generateReferences(vs *istioclientv1beta1.VirtualService, fieldPath *field.Path) ([]gatewayv1.ParentReference, []*gatewayv1alpha2.ReferenceGrant) {
	var (
		parentRefs      []gatewayv1.ParentReference
		referenceGrants []*gatewayv1alpha2.ReferenceGrant
	)

	for _, allowedGateway := range vs.Spec.GetGateways() {
		gwNamespace, gwName, ok := strings.Cut(allowedGateway, "/")
		if !ok {
			gwNamespace, gwName = vs.Namespace, allowedGateway
		}

		gateway := types.NamespacedName{
			Namespace: gwNamespace,
			Name:      gwName,
		}

		if !c.isVirtualServiceAllowedForGateway(gateway, vs, fieldPath.Child("Spec", "Gateways").Key(allowedGateway)) {
			continue
		}

		g := gatewayv1.Group(common.GatewayGVK.Group)
		k := gatewayv1.Kind(common.GatewayGVK.Kind)
		ns := gatewayv1.Namespace(gateway.Namespace)

		parentRef := gatewayv1.ParentReference{
			Group: &g,
			Kind:  &k,
			Name:  gatewayv1.ObjectName(gateway.Name),
		}

		if gateway.Namespace != vs.Namespace {
			parentRef.Namespace = &ns

			referenceGrants = append(referenceGrants, c.generateReferenceGrant(generateReferenceGrantsParams{
				gateway:       gateway,
				fromNamespace: vs.Namespace,
				forHTTPRoute:  vs.Spec.GetHttp() != nil,
				forTLSRoute:   vs.Spec.GetTls() != nil,
				forTCPRoute:   vs.Spec.GetTcp() != nil,
			}))
		}

		parentRefs = append(parentRefs, parentRef)
	}

	return parentRefs, referenceGrants
}

type generateReferenceGrantsParams struct {
	gateway                                types.NamespacedName
	fromNamespace                          string
	forHTTPRoute, forTLSRoute, forTCPRoute bool
}

func (c *converter) generateReferenceGrant(params generateReferenceGrantsParams) *gatewayv1alpha2.ReferenceGrant {
	var fromGrants []gatewayv1alpha2.ReferenceGrantFrom

	if params.forHTTPRoute {
		fromGrants = append(fromGrants, gatewayv1alpha2.ReferenceGrantFrom{
			Group:     gatewayv1.Group(common.HTTPRouteGVK.Group),
			Kind:      gatewayv1.Kind(common.HTTPRouteGVK.Kind),
			Namespace: gatewayv1.Namespace(params.fromNamespace),
		})
	}

	if params.forTLSRoute {
		fromGrants = append(fromGrants, gatewayv1alpha2.ReferenceGrantFrom{
			Group:     gatewayv1.Group(common.TLSRouteGVK.Group),
			Kind:      gatewayv1.Kind(common.TLSRouteGVK.Kind),
			Namespace: gatewayv1.Namespace(params.fromNamespace),
		})
	}

	if params.forTCPRoute {
		fromGrants = append(fromGrants, gatewayv1alpha2.ReferenceGrantFrom{
			Group:     gatewayv1.Group(common.TCPRouteGVK.Group),
			Kind:      gatewayv1.Kind(common.TCPRouteGVK.Kind),
			Namespace: gatewayv1.Namespace(params.fromNamespace),
		})
	}

	gwName := gatewayv1.ObjectName(params.gateway.Name)

	return &gatewayv1alpha2.ReferenceGrant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: common.ReferenceGrantGVK.GroupVersion().String(),
			Kind:       common.ReferenceGrantGVK.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: params.gateway.Namespace,
			Name:      fmt.Sprintf("generated-reference-grant-from-%v-to-%v", params.fromNamespace, params.gateway.Namespace),
		},
		Spec: gatewayv1alpha2.ReferenceGrantSpec{
			From: fromGrants,
			To: []gatewayv1alpha2.ReferenceGrantTo{
				{
					Group: gatewayv1.Group(common.GatewayGVK.Group),
					Kind:  gatewayv1.Kind(common.GatewayGVK.Kind),
					Name:  &gwName,
				},
			},
		},
	}
}

func parseK8SServiceFromDomain(domain string, fallbackNamespace string) (string, string) {
	ns := "default"
	if fallbackNamespace != "" {
		ns = fallbackNamespace
	}

	idx := strings.Index(domain, ".svc")
	if idx == -1 {
		return domain, ns
	}

	name, namespace, ok := strings.Cut(domain[:idx], ".")
	if !ok {
		return domain[:idx], ns
	}
	return name, namespace
}

func destination2backendObjRef(destination *istiov1beta1.Destination, vsNamespace string, fieldPath *field.Path) *gatewayv1.BackendObjectReference {
	if destination == nil {
		klog.Infof("destination is nil: %v", fieldPath)
		return nil
	}

	if destination.GetSubset() != "" {
		klog.Infof("ignoring field: %v", fieldPath.Child("Destination", "Subset"))
	}

	serviceName, serviceNamespace := parseK8SServiceFromDomain(destination.GetHost(), vsNamespace)

	namespace := gatewayv1.Namespace(serviceNamespace)

	var port *gatewayv1.PortNumber
	if destinationPort := destination.GetPort(); destinationPort != nil {
		p := gatewayv1.PortNumber(destinationPort.GetNumber())
		port = &p
	}

	// empty group&kind mean core/Service
	return &gatewayv1.BackendObjectReference{
		Name:      gatewayv1.ObjectName(serviceName),
		Namespace: &namespace,
		Port:      port,
	}
}

func makeHeaderFilter(headers map[string]string) []gatewayv1.HTTPHeader {
	if headers == nil {
		return nil
	}

	res := make([]gatewayv1.HTTPHeader, 0, len(headers))

	for header, value := range headers {
		res = append(res, gatewayv1.HTTPHeader{
			Name:  gatewayv1.HTTPHeaderName(header),
			Value: value,
		})
	}

	return res
}

// checks if host overlaps with any of the hosts
func matchAny(hosts []string, host string) bool {
	for _, h := range hosts {
		if matches(host, h) {
			return true
		}
	}
	return false
}

// Matches returns true if this hostname overlaps with the other hostname. Names overlap if:
// - they're fully resolved (i.e. not wildcarded) and match exactly (i.e. an exact string match)
// - one or both are wildcarded (e.g. "*.foo.com"), in which case we use wildcard resolution rules
// to determine if n is covered by o or o is covered by n.
// e.g.:
//
// matches("foo.com", "foo.com")   = true
// matches("foo.com", "bar.com")   = false
// matches("*.com", "foo.com")     = true
// matches("bar.com", "*.com")     = true
// matches("*.foo.com", "foo.com") = false
// matches("*", "foo.com")         = true
// matches("*", "*.com")           = true
// taken from: https://github.com/istio/istio/blob/2fd7c2719bd3c5c9bb4aaf34b8f4565229c3035b/pkg/config/host/name.go#L37
func matches(h1, h2 string) bool {
	h1Wildcard, h2Wildcard := isWildCarded(h1), isWildCarded(h2)

	if h1Wildcard {
		if h2Wildcard {
			// both h1 and h2 are wildcards
			if len(h1) < len(h2) {
				return strings.HasSuffix(h2[1:], h1[1:])
			}
			return strings.HasSuffix(h1[1:], h2[1:])
		}
		// only h1 is wildcard
		return strings.HasSuffix(h2, h1[1:])
	}

	if h2Wildcard {
		// only h2 is wildcard
		return strings.HasSuffix(h1, h2[1:])
	}

	// both are non-wildcards, so do normal string comparison
	return h1 == h2
}

func isWildCarded(hostname string) bool {
	return len(hostname) > 0 && hostname[0] == '*'
}
