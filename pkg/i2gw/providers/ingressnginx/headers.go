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

package ingressnginx

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func headerModifierFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR, cms map[types.NamespacedName]map[string]string) {
	for _, httpRouteContext := range ir.HTTPRoutes {
		for i := range httpRouteContext.HTTPRoute.Spec.Rules {
			if i >= len(httpRouteContext.RuleBackendSources) {
				continue
			}
			sources := httpRouteContext.RuleBackendSources[i]

			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				panic("No non-canary ingress found")
			}

			headersToSet := make(map[string]string)

			// 1. upstream-vhost -> Host header
			if val, ok := ingress.Annotations[UpstreamVhostAnnotation]; ok && val != "" {
				headersToSet["Host"] = val
			}

			// 2. connection-proxy-header -> Connection header
			if val, ok := ingress.Annotations[ConnectionProxyHeaderAnnotation]; ok && val != "" {
				headersToSet["Connection"] = val
			}

			// 3. custom-headers -> Warn unsupported
			// TODO: implement custom-headers annotation.
			if val, ok := ingress.Annotations[CustomHeadersAnnotation]; ok {
				parts := strings.SplitN(val, "/", 2)
				cmNamespace := parts[0]
				cmName := parts[1]
				cmKey := types.NamespacedName{Namespace: cmNamespace, Name: cmName}
				customHeaders, exists := cms[cmKey]
				if !exists {
					notify(notifications.WarningNotification, fmt.Sprintf("Ingress %s/%s references ConfigMap %s/%s for custom headers which does not exist.", ingress.Namespace, ingress.Name, cmNamespace, cmName), &httpRouteContext.HTTPRoute)
				} else {
					for headerName, headerValue := range customHeaders {
						headersToSet[headerName] = headerValue
					}
				}
			}

			if len(headersToSet) > 0 {
				applyHeaderModifiers(&httpRouteContext.HTTPRoute, i, headersToSet)
			}
		}
	}
}

func applyHeaderModifiers(httpRoute *gatewayv1.HTTPRoute, ruleIndex int, headersToSet map[string]string) {
	// Find existing RequestHeaderModifier filter or create new one
	var filter *gatewayv1.HTTPRouteFilter
	for j, f := range httpRoute.Spec.Rules[ruleIndex].Filters {
		if f.Type == gatewayv1.HTTPRouteFilterRequestHeaderModifier && f.RequestHeaderModifier != nil {
			filter = &httpRoute.Spec.Rules[ruleIndex].Filters[j]
			break
		}
	}

	if filter == nil {
		f := gatewayv1.HTTPRouteFilter{
			Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
				Set: []gatewayv1.HTTPHeader{},
			},
		}
		httpRoute.Spec.Rules[ruleIndex].Filters = append(httpRoute.Spec.Rules[ruleIndex].Filters, f)
		filter = &httpRoute.Spec.Rules[ruleIndex].Filters[len(httpRoute.Spec.Rules[ruleIndex].Filters)-1]
	}

	for name, value := range headersToSet {
		// Used standard append as suggested in PR review
		filter.RequestHeaderModifier.Set = append(filter.RequestHeaderModifier.Set, gatewayv1.HTTPHeader{
			Name:  gatewayv1.HTTPHeaderName(name),
			Value: value,
		})
		notify(notifications.InfoNotification, fmt.Sprintf("Applied header modifier %s: %s to rule %d of route %s/%s", name, value, ruleIndex, httpRoute.Namespace, httpRoute.Name), httpRoute)
	}
}
