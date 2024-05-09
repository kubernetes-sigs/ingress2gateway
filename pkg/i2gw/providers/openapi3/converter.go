/*
Copyright 2024 The Kubernetes Authors.

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

package openapi3

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

const (
	HostWildcard   = "*"
	HostSeparator  = ","
	ParamSeparator = ","

	HTTPRouteRulesMax      = 16
	HTTPRouteMatchesMax    = 8
	HTTPRouteMatchesMaxMax = HTTPRouteRulesMax * HTTPRouteMatchesMax
)

var uriRegexp = regexp.MustCompile(`^((https?)://([^/]+))?(/.*)?$`) // [_][1] = scheme, [_][2] = host, [_][3] = path

type Converter interface {
	Convert(Storage) (i2gw.GatewayResources, field.ErrorList)
}

// NewConverter returns a converter of OpenAPI Specifications 3.x from a storage into Gateway API resources.
func NewConverter(conf *i2gw.ProviderConf) Converter {
	converter := &converter{
		namespace:    conf.Namespace,
		tlsSecretRef: &types.NamespacedName{},
		backendRef:   &types.NamespacedName{},
	}

	if ps := conf.ProviderSpecific[ProviderName]; ps != nil {
		converter.gatewayClassName = ps[GatewayClassFlag]
		converter.tlsSecretRef = toNamespacedName(ps[TlsSecretFlag])
		converter.backendRef = toNamespacedName(ps[BackendFlag])
	}

	return converter
}

type converter struct {
	namespace        string
	gatewayClassName string
	tlsSecretRef     *types.NamespacedName
	backendRef       *types.NamespacedName
}

var _ Converter = &converter{}

func (c *converter) Convert(storage Storage) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources := i2gw.GatewayResources{
		Gateways:        make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes:      make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		ReferenceGrants: make(map[types.NamespacedName]gatewayv1beta1.ReferenceGrant),
	}

	var errors field.ErrorList

	for _, spec := range storage.GetResources() {
		httpRoutes, gateways := c.toHTTPRoutesAndGateways(spec, errors)
		for _, httpRoute := range httpRoutes {
			gatewayResources.HTTPRoutes[types.NamespacedName{Name: httpRoute.GetName(), Namespace: httpRoute.GetNamespace()}] = httpRoute
		}
		if referenceGrant := c.buildHTTPRouteBackendReferenceGrant(); referenceGrant != nil {
			gatewayResources.ReferenceGrants[types.NamespacedName{Name: referenceGrant.GetName(), Namespace: referenceGrant.GetNamespace()}] = *referenceGrant
		}
		for _, gateway := range gateways {
			gatewayResources.Gateways[types.NamespacedName{Name: gateway.GetName(), Namespace: gateway.GetNamespace()}] = gateway
			if referenceGrant := c.buildGatewayTlsSecretReferenceGrant(gateway); referenceGrant != nil {
				gatewayResources.ReferenceGrants[types.NamespacedName{Name: referenceGrant.GetName(), Namespace: referenceGrant.GetNamespace()}] = *referenceGrant
			}
		}
	}

	return gatewayResources, errors
}

type httpRouteRuleMatcher struct {
	path    string
	method  string
	headers string
	params  string
}

type httpRouteRuleMatchers []httpRouteRuleMatcher

func (m httpRouteRuleMatchers) Len() int      { return len(m) }
func (m httpRouteRuleMatchers) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m httpRouteRuleMatchers) Less(i, j int) bool {
	if m[i].path != m[j].path {
		return m[i].path < m[j].path
	}
	return m[i].method < m[j].method
}

type httpRouteMatcher struct {
	protocol string
	host     string
	httpRouteRuleMatcher
}

func (c *converter) toHTTPRoutesAndGateways(spec *openapi3.T, errors field.ErrorList) ([]gatewayv1.HTTPRoute, []gatewayv1.Gateway) {
	var matchers []httpRouteMatcher

	servers := spec.Servers
	if len(servers) == 0 {
		servers = openapi3.Servers{{URL: "/"}}
	}

	paths := spec.Paths.Map()
	for _, relativePath := range spec.Paths.InMatchingOrder() {
		pathItem := paths[relativePath]
		matchers = append(matchers, pathItemToHTTPMatchers(pathItem, relativePath, servers, errors)...)
	}

	listenersByHTTPRouteRuleMatcher := make(map[httpRouteRuleMatcher][]string)
	for _, matcher := range matchers {
		listener := fmt.Sprintf("%s://%s", matcher.protocol, matcher.host)
		listenersByHTTPRouteRuleMatcher[matcher.httpRouteRuleMatcher] = append(listenersByHTTPRouteRuleMatcher[matcher.httpRouteRuleMatcher], listener)
	}

	var listenerGroups []string
	httpRouteRuleMatchersByListeners := make(map[string]httpRouteRuleMatchers)
	for matcher, listeners := range listenersByHTTPRouteRuleMatcher {
		group := strings.Join(listeners, HostSeparator)
		if _, exists := httpRouteRuleMatchersByListeners[group]; !exists {
			listenerGroups = append(listenerGroups, group)
		}
		httpRouteRuleMatchersByListeners[group] = append(httpRouteRuleMatchersByListeners[group], matcher)
	}

	// sort listener groups for deterministic output
	sort.Strings(listenerGroups)

	gatewayName := "gateway-1" // TODO: name the gateway after the spec
	gateway := gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: gatewayName,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(c.gatewayClassName),
		},
	}
	gateway.SetGroupVersionKind(common.GatewayGVK)
	if c.namespace != "" {
		gateway.SetNamespace(c.namespace)
	}

	uniqueListeners := make(map[string]struct{})
	for _, group := range listenerGroups {
		listeners := lo.Filter(strings.Split(group, HostSeparator), func(listener string, _ int) bool {
			_, exists := uniqueListeners[listener]
			if !exists {
				uniqueListeners[listener] = struct{}{}
			}
			return !exists
		})
		gateway.Spec.Listeners = append(gateway.Spec.Listeners, lo.Map(listeners, c.toListener)...) // TODO: gateways cannot have more than 64 listeners
	}

	var routes []gatewayv1.HTTPRoute

	i := 0
	for _, group := range listenerGroups {
		listeners := strings.Split(group, HostSeparator)
		hosts := lo.Map(listeners, uriToHostname)
		matchers := httpRouteRuleMatchersByListeners[group]

		var listenerName gatewayv1.SectionName
		if len(uniqueListeners) > 1 && len(listeners) == 1 {
			listenerName, _, _ = toListenerName(listeners[0])
		}

		// sort hostnames and matchers for deterministic output inside each route object
		sort.Sort(matchers)
		sort.Strings(hosts)
		hosts = slices.Compact(hosts)

		nMatchers := len(matchers)
		nRoutes := nMatchers / HTTPRouteMatchesMaxMax
		if nMatchers%HTTPRouteMatchesMaxMax != 0 {
			nRoutes++
		}
		for j := 0; j < nRoutes; j++ {
			routeName := fmt.Sprintf("route-%d-%d", i+1, j+1)
			// TODO: name the route after the spec
			last := (j + 1) * HTTPRouteMatchesMaxMax
			if last > nMatchers {
				last = nMatchers
			}
			routes = append(routes, c.toHTTPRoute(routeName, gatewayName, listenerName, hosts, matchers[j*HTTPRouteMatchesMaxMax:last]))
		}
		i++
	}

	return routes, []gatewayv1.Gateway{gateway}
}

func (c *converter) toListener(protocolAndHostname string, _ int) gatewayv1.Listener {
	name, protocol, hostname := toListenerName(protocolAndHostname)

	listener := gatewayv1.Listener{
		Name:     name,
		Protocol: gatewayv1.ProtocolType(strings.ToUpper(protocol)),
		Hostname: common.PtrTo(gatewayv1.Hostname(hostname)),
	}

	switch protocol {
	case "http":
		listener.Port = 80
	case "https":
		listener.Port = 443

		tlsSecretRef := gatewayv1.SecretObjectReference{
			Name: gatewayv1.ObjectName(c.tlsSecretRef.Name),
		}
		if c.tlsSecretRef.Namespace != "" {
			tlsSecretRef.Namespace = common.PtrTo(gatewayv1.Namespace(c.tlsSecretRef.Namespace))
		}

		listener.TLS = &gatewayv1.GatewayTLSConfig{
			CertificateRefs: []gatewayv1.SecretObjectReference{tlsSecretRef},
		}
	}

	return listener
}

func toListenerName(protocolAndHostname string) (listenerName gatewayv1.SectionName, protocol string, hostname string) {
	protocol = "http"
	hostname = HostWildcard

	if s := uriRegexp.FindAllStringSubmatch(protocolAndHostname, 1); len(s) > 0 {
		if s[0][2] != "" {
			protocol = s[0][2]
		}
		if s[0][3] != "" {
			hostname = s[0][3]
		}
	}

	var listenerNamePrefix string
	if hostname != HostWildcard {
		listenerNamePrefix = fmt.Sprintf("%s-", common.NameFromHost(hostname))
	}

	return gatewayv1.SectionName(listenerNamePrefix + protocol), protocol, hostname
}

func (c *converter) toHTTPRoute(name, gatewayName string, listenerName gatewayv1.SectionName, hostnames []string, matchers httpRouteRuleMatchers) gatewayv1.HTTPRoute {
	parentRef := gatewayv1.ParentReference{Name: gatewayv1.ObjectName(gatewayName)}
	if listenerName != "" {
		parentRef.SectionName = common.PtrTo(listenerName)
	}
	route := gatewayv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1.GroupVersion.String(),
			Kind:       "HTTPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parentRef},
			},
			Rules: c.toHTTPRouteRules(matchers),
		},
	}
	if c.namespace != "" {
		route.SetNamespace(c.namespace)
	}
	if len(hostnames) > 1 || !slices.Contains(hostnames, HostWildcard) {
		route.Spec.Hostnames = lo.Map(hostnames, toGatewayAPIHostname)
	}
	return route
}

func (c *converter) toHTTPRouteRules(matchers httpRouteRuleMatchers) []gatewayv1.HTTPRouteRule {
	var rules []gatewayv1.HTTPRouteRule
	nMatches := len(matchers)
	nRules := nMatches / HTTPRouteMatchesMax
	if len(matchers)%HTTPRouteMatchesMax != 0 {
		nRules++
	}

	backendObjectReference := gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(c.backendRef.Name)}
	if c.backendRef.Namespace != "" {
		backendObjectReference.Namespace = common.PtrTo(gatewayv1.Namespace(c.backendRef.Namespace))
	}
	for i := 0; i < nRules; i++ {
		rule := gatewayv1.HTTPRouteRule{}
		offfset := i * HTTPRouteMatchesMax
		for j := 0; j < HTTPRouteMatchesMax && offfset+j < nMatches; j++ {
			matcher := matchers[offfset+j]
			ruleMatch := gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  common.PtrTo(gatewayv1.PathMatchExact),
					Value: &matcher.path,
				},
				Method: common.PtrTo(gatewayv1.HTTPMethod(matcher.method)),
			}
			if matcher.headers != "" {
				ruleMatch.Headers = lo.Map(strings.Split(matcher.headers, ParamSeparator), func(header string, _ int) gatewayv1.HTTPHeaderMatch {
					return gatewayv1.HTTPHeaderMatch{
						Name: gatewayv1.HTTPHeaderName(header),
						Type: common.PtrTo(gatewayv1.HeaderMatchExact),
					}
				})
			}
			if matcher.params != "" {
				ruleMatch.QueryParams = lo.Map(strings.Split(matcher.params, ParamSeparator), func(param string, _ int) gatewayv1.HTTPQueryParamMatch {
					return gatewayv1.HTTPQueryParamMatch{
						Name: gatewayv1.HTTPHeaderName(param),
						Type: common.PtrTo(gatewayv1.QueryParamMatchExact),
					}
				})
			}
			rule.Matches = append(rule.Matches, ruleMatch)
		}

		rule.BackendRefs = []gatewayv1.HTTPBackendRef{
			gatewayv1.HTTPBackendRef{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: backendObjectReference,
				},
			},
		}
		rules = append(rules, rule)
	}
	return rules
}

func pathItemToHTTPMatchers(pathItem *openapi3.PathItem, relativePath string, servers openapi3.Servers, errors field.ErrorList) []httpRouteMatcher {
	var matchers []httpRouteMatcher

	if len(pathItem.Servers) > 0 {
		servers = pathItem.Servers
	}

	operations := map[string]*openapi3.Operation{
		"CONNECT": pathItem.Connect,
		"DELETE":  pathItem.Delete,
		"GET":     pathItem.Get,
		"HEAD":    pathItem.Head,
		"OPTIONS": pathItem.Options,
		"PATCH":   pathItem.Patch,
		"POST":    pathItem.Post,
		"PUT":     pathItem.Put,
		"TRACE":   pathItem.Trace,
	}

	for method, operation := range operations {
		if operation == nil {
			continue
		}
		matchers = append(matchers, operationToHTTPMatchers(operation, relativePath, method, pathItem.Parameters, servers, errors)...)
	}

	return matchers
}

func operationToHTTPMatchers(operation *openapi3.Operation, relativePath string, method string, parameters openapi3.Parameters, servers openapi3.Servers, errors field.ErrorList) []httpRouteMatcher {
	if operation.Servers != nil && len(*operation.Servers) > 0 {
		servers = *operation.Servers
	}

	if operation.Parameters != nil {
		parameters = operation.Parameters
	}

	var expandedServers []openapi3.Server
	for _, server := range servers {
		expandedServers = append(expandedServers, expandServerVariables(*server)...)
	}

	return lo.Map(expandedServers, toHTTPMatcher(relativePath, method, parameters, errors))
}

func toHTTPMatcher(relativePath string, method string, parameters openapi3.Parameters, errors field.ErrorList) func(server openapi3.Server, _ int) httpRouteMatcher {
	paramNameFunc := func(in string) func(p *openapi3.ParameterRef, _ int) (string, bool) {
		return func(p *openapi3.ParameterRef, _ int) (string, bool) {
			if p.Value != nil && p.Value.Required && p.Value.In == in {
				return p.Value.Name, true
			}
			return "", false
		}
	}
	headers := strings.Join(lo.FilterMap(parameters, paramNameFunc("header")), ParamSeparator)
	params := strings.Join(lo.FilterMap(parameters, paramNameFunc("query")), ParamSeparator)

	return func(server openapi3.Server, _ int) httpRouteMatcher {
		basePath, err := server.BasePath()
		if err != nil {
			errors = append(errors, field.Invalid(field.NewPath("servers"), server, err.Error()))
		}
		if basePath == "/" {
			basePath = ""
		}
		protocol := "http"
		if s := uriRegexp.FindAllStringSubmatch(server.URL, 1); len(s) > 0 && s[0][2] != "" {
			protocol = s[0][2]
		}
		return httpRouteMatcher{
			protocol: strings.ToLower(protocol),
			host:     uriToHostname(server.URL, 0),
			httpRouteRuleMatcher: httpRouteRuleMatcher{
				path:    basePath + relativePath,
				method:  method,
				headers: headers,
				params:  params,
			},
		}
	}
}

func (c *converter) buildHTTPRouteBackendReferenceGrant() *gatewayv1beta1.ReferenceGrant {
	return c.buildReferenceGrant(common.HTTPRouteGVK, gatewayv1.Kind("Service"), c.backendRef)
}

func (c *converter) buildGatewayTlsSecretReferenceGrant(gateway gatewayv1.Gateway) *gatewayv1beta1.ReferenceGrant {
	if slices.IndexFunc(gateway.Spec.Listeners, func(listener gatewayv1.Listener) bool { return listener.TLS != nil }) == -1 {
		return nil
	}
	return c.buildReferenceGrant(common.GatewayGVK, gatewayv1.Kind("Secret"), c.tlsSecretRef)
}

func (c *converter) buildReferenceGrant(fromGVK schema.GroupVersionKind, toKind gatewayv1.Kind, toRef *types.NamespacedName) *gatewayv1beta1.ReferenceGrant {
	if c.namespace == "" || toRef.Namespace == "" {
		return nil
	}
	rg := &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("from-%s-to-%s-%s", c.namespace, strings.ToLower(string(toKind)), toRef.Name),
			Namespace: toRef.Namespace,
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1.Group(fromGVK.Group),
					Kind:      gatewayv1.Kind(fromGVK.Kind),
					Namespace: gatewayv1.Namespace(c.namespace),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Kind: toKind,
					Name: common.PtrTo(gatewayv1.ObjectName(toRef.Name)),
				},
			},
		},
	}
	rg.SetGroupVersionKind(common.ReferenceGrantGVK)
	return rg
}

func expandNonEnumServerVariables(server openapi3.Server) openapi3.Server {
	if len(server.Variables) == 0 {
		return server
	}
	// non-enum variables
	uri := server.URL
	variables := make(map[string]*openapi3.ServerVariable)
	for name, svar := range server.Variables {
		if len(svar.Enum) > 0 {
			variables[name] = svar
			continue
		}
		uri = strings.ReplaceAll(uri, "{"+name+"}", svar.Default)
	}
	return openapi3.Server{
		URL:       uri,
		Variables: variables,
	}
}

func expandServerVariables(server openapi3.Server) []openapi3.Server {
	servers := []openapi3.Server{expandNonEnumServerVariables(server)}
	for {
		var newServers []openapi3.Server
		for _, server := range servers {
			if len(server.Variables) == 0 {
				newServers = append(newServers, server)
				continue
			}
			var name string
			var svar *openapi3.ServerVariable
			for name, svar = range server.Variables {
				break
			}
			var uris []string
			for _, enum := range svar.Enum {
				uri := strings.ReplaceAll(server.URL, "{"+name+"}", enum)
				uris = append(uris, uri)
			}
			variables := make(map[string]*openapi3.ServerVariable, len(server.Variables)-1)
			for n, v := range server.Variables {
				if n != name {
					variables[n] = v
				}
			}
			for _, uri := range uris {
				newServers = append(newServers, openapi3.Server{
					URL:       uri,
					Variables: variables,
				})
			}
		}
		servers = newServers
		if slices.IndexFunc(servers, func(server openapi3.Server) bool { return len(server.Variables) > 0 }) == -1 {
			break
		}
	}
	return servers
}

func uriToHostname(uri string, _ int) string {
	host := HostWildcard
	if s := uriRegexp.FindAllStringSubmatch(uri, 1); len(s) > 0 && s[0][3] != "" {
		host = s[0][3]
	}
	return host
}

func toGatewayAPIHostname(hostname string, _ int) gatewayv1.Hostname {
	return gatewayv1.Hostname(hostname)
}

func toNamespacedName(s string) *types.NamespacedName {
	if s == "" {
		return nil
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 1 {
		return &types.NamespacedName{Name: parts[0]}
	}
	return &types.NamespacedName{Namespace: parts[0], Name: parts[1]}
}
