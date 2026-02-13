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

package annotations

import (
	"log/slog"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// RewriteTargetFeature converts nginx.org/rewrites annotation to URLRewrite filter
func RewriteTargetFeature(_ *slog.Logger, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			rewriteValue, exists := rule.Ingress.Annotations[nginxRewritesAnnotation]
			if !exists || rewriteValue == "" {
				continue
			}

			rewriteRules := parseRewriteRules(rewriteValue)
			if len(rewriteRules) == 0 {
				continue
			}

			// Get the HTTPRoute for this rule group
			key := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRouteContext, ok := ir.HTTPRoutes[key]
			if !ok {
				continue
			}

			for i := range httpRouteContext.HTTPRoute.Spec.Rules {
				for _, path := range rule.IngressRule.HTTP.Paths {
					serviceName := path.Backend.Service.Name
					if rewritePath, hasRewrite := rewriteRules[serviceName]; hasRewrite {
						filter := gatewayv1.HTTPRouteFilter{
							Type: gatewayv1.HTTPRouteFilterURLRewrite,
							URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
								Path: &gatewayv1.HTTPPathModifier{
									Type:               gatewayv1.PrefixMatchHTTPPathModifier,
									ReplacePrefixMatch: ptr.To(rewritePath),
								},
							},
						}

						if httpRouteContext.HTTPRoute.Spec.Rules[i].Filters == nil {
							httpRouteContext.HTTPRoute.Spec.Rules[i].Filters = []gatewayv1.HTTPRouteFilter{}
						}
						httpRouteContext.HTTPRoute.Spec.Rules[i].Filters = append(httpRouteContext.HTTPRoute.Spec.Rules[i].Filters, filter)
					}
				}
			}

			// Update the HTTPRoute in the IR
			ir.HTTPRoutes[key] = httpRouteContext
		}
	}

	return errs
}

// parseRewriteRules parses nginx.org/rewrites annotation format
// NIC format: "serviceName=service rewrite=path;serviceName2=service2 rewrite=path2"
func parseRewriteRules(rewriteValue string) map[string]string {
	rules := make(map[string]string)

	if rewriteValue == "" {
		return rules
	}

	// Split by semicolon for each rule
	parts := strings.Split(rewriteValue, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Expect format: serviceName=service rewrite=rewrite
		serviceIdx := strings.Index(part, "=")
		rewriteIdx := strings.Index(part, " rewrite=")
		if serviceIdx == -1 || rewriteIdx == -1 || rewriteIdx <= serviceIdx {
			continue
		}

		serviceName := strings.TrimSpace(part[serviceIdx+1 : rewriteIdx])
		rewritePath := strings.TrimSpace(part[rewriteIdx+9:])

		if serviceName != "" && rewritePath != "" {
			rules[serviceName] = rewritePath
		}
	}

	return rules
}
