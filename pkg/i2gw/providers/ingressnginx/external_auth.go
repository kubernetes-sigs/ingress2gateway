/*
Copyright 2025 The Kubernetes Authors.

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

package ingressnginx

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	extAuthUrlAnnotation             = "nginx.ingress.kubernetes.io/auth-url"
	extAuthResponseHeadersAnnotation = "nginx.ingress.kubernetes.io/auth-response-headers"
)

// authServiceInfo holds the parsed auth service information from a single Ingress
type authServiceInfo struct {
	serviceName string
	namespace   string
	port        int32
	path        string
}

// parseAuthUrl parses the auth-url annotation and extracts service information.
// Only Kubernetes internal services with .svc.cluster.local suffix are supported.
// Expected format: http://service-name.namespace.svc.cluster.local:port/path
func parseAuthUrl(ingress *networkingv1.Ingress) (*authServiceInfo, error) {
	parsedUrl, err := url.Parse(ingress.Annotations[extAuthUrlAnnotation])
	if err != nil {
		return nil, fmt.Errorf("invalid auth url: %w", err)
	}

	host := parsedUrl.Hostname()
	if host == "" {
		return nil, fmt.Errorf("auth url must contain a hostname")
	}

	// Validate that the host ends with .svc.cluster.local
	if !strings.HasSuffix(host, ".svc.cluster.local") {
		return nil, fmt.Errorf("auth url must use Kubernetes service FQDN with .svc.cluster.local suffix, got %q", host)
	}

	// Parse service name and namespace from FQDN
	// Expected format: service-name.namespace.svc.cluster.local
	parts := strings.Split(host, ".")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid service FQDN format: %q (expected: service.namespace.svc.cluster.local)", host)
	}

	serviceName := parts[0]
	namespace := parts[1]

	// Parse port
	port := int32(80)
	if parsedUrl.Port() != "" {
		p, err := strconv.ParseInt(parsedUrl.Port(), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid port in auth url: %w", err)
		}
		port = int32(p)
	} else if parsedUrl.Scheme == "https" {
		port = int32(443)
	}

	return &authServiceInfo{
		serviceName: serviceName,
		namespace:   namespace,
		port:        port,
		path:        parsedUrl.Path,
	}, nil
}

// parseResponseHeaders parses the comma-separated list of response headers
func parseResponseHeaders(ingress *networkingv1.Ingress) []string {
	headers := ingress.Annotations[extAuthResponseHeadersAnnotation]
	if headers == "" {
		return nil
	}

	parts := strings.Split(headers, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func externalAuthFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, pir *providerir.ProviderIR, eir *emitterir.EmitterIR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errList field.ErrorList

	for _, rg := range ruleGroups {
		key := types.NamespacedName{Namespace: rg.Namespace, Name: common.RouteName(rg.Name, rg.Host)}

		// Get RuleBackendSources from Provider IR
		providerHTTPRouteContext, ok := pir.HTTPRoutes[key]
		if !ok {
			continue
		}

		emitterHTTPRouteContext, ok := eir.HTTPRoutes[key]
		if !ok {
			continue
		}

		for ruleIdx, backendSources := range providerHTTPRouteContext.RuleBackendSources {
			if ruleIdx >= len(emitterHTTPRouteContext.HTTPRoute.Spec.Rules) {
				errList = append(errList, field.InternalError(
					field.NewPath("httproute", emitterHTTPRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx),
					fmt.Errorf("rule index %d exceeds available rules", ruleIdx),
				))
				continue
			}

			var authConfig *emitterir.ExternalAuthConfig
			var authSourceIngress *networkingv1.Ingress

			for _, source := range backendSources {
				if source.Ingress == nil {
					continue
				}

				if source.Ingress.Annotations[extAuthUrlAnnotation] != "" {
					if authConfig != nil {
						errList = append(errList, field.Invalid(
							field.NewPath("httproute", emitterHTTPRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
							fmt.Sprintf("ingresses %s/%s and %s/%s", authSourceIngress.Namespace, authSourceIngress.Name, source.Ingress.Namespace, source.Ingress.Name),
							"at most one external auth is allowed per rule",
						))
						continue
					}

					authInfo, err := parseAuthUrl(source.Ingress)
					if err != nil {
						errList = append(errList, field.Invalid(
							field.NewPath("ingress", source.Ingress.Namespace, source.Ingress.Name, "metadata", "annotations"),
							source.Ingress.Annotations,
							fmt.Sprintf("failed to parse auth url: %v", err),
						))
						continue
					}

					authConfig = &emitterir.ExternalAuthConfig{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Group:     ptr.To(gatewayv1.Group("")),
							Kind:      ptr.To(gatewayv1.Kind("Service")),
							Name:      gatewayv1.ObjectName(authInfo.serviceName),
							Namespace: ptr.To(gatewayv1.Namespace(authInfo.namespace)),
							Port:      ptr.To(authInfo.port),
						},
						Protocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
						Path:     authInfo.path,
					}
					if source.Ingress.Annotations[extAuthResponseHeadersAnnotation] != "" {
						responseHeaders := parseResponseHeaders(source.Ingress)
						authConfig.AllowedResponseHeaders = responseHeaders
					}

					authSourceIngress = source.Ingress

					if emitterHTTPRouteContext.ExtAuth == nil {
						emitterHTTPRouteContext.ExtAuth = make(map[int]*emitterir.ExternalAuthConfig)
					}
					emitterHTTPRouteContext.ExtAuth[ruleIdx] = authConfig
				}
			}
		}
		eir.HTTPRoutes[key] = emitterHTTPRouteContext
	}

	if len(errList) > 0 {
		return errList
	}
	return nil
}
