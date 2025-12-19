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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func headerModifierFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
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

			_, hasRewriteTarget := ingress.Annotations[RewriteTargetAnnotation]

			// 1. x-forwarded-prefix
			// This annotation only works if rewrite-target is also present.
			if val, ok := ingress.Annotations[XForwardedPrefixAnnotation]; ok && val != "" && hasRewriteTarget {
				headersToSet["X-Forwarded-Prefix"] = val
			}

			// 2. upstream-vhost -> Host header
			if val, ok := ingress.Annotations[UpstreamVhostAnnotation]; ok && val != "" {
				headersToSet["Host"] = val
			}

			// 3. connection-proxy-header -> Connection header
			if val, ok := ingress.Annotations[ConnectionProxyHeaderAnnotation]; ok && val != "" {
				headersToSet["Connection"] = val
			}

			// 4. custom-headers -> Warn unsupported
			if _, ok := ingress.Annotations[CustomHeadersAnnotation]; ok {
				notify(notifications.WarningNotification, fmt.Sprintf("Ingress %s/%s uses 'nginx.ingress.kubernetes.io/custom-headers' which is not supported as it requires cluster access to read ConfigMaps.", ingress.Namespace, ingress.Name), &httpRouteContext.HTTPRoute)
			}

			if len(headersToSet) > 0 {
				applyHeaderModifiers(&httpRouteContext.HTTPRoute, i, headersToSet)
			}
		}
	}
	return nil
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
