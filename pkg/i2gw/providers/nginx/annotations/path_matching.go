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
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// PathRegexFeature converts nginx.org/path-regex annotation to regex path matching
func PathRegexFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *provider_intermediate.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	// Valid values for path-regex annotation
	var validPathRegexValues = map[string]struct{}{
		"true":             {},
		"case_sensitive":   {},
		"case_insensitive": {},
		"exact":            {},
	}

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			pathRegex, exists := rule.Ingress.Annotations[nginxPathRegexAnnotation]
			if !exists || pathRegex == "" {
				continue
			}

			if _, valid := validPathRegexValues[pathRegex]; !valid {
				continue
			}

			// Determine the appropriate path match type based on the annotation value
			var pathMatchType gatewayv1.PathMatchType
			if pathRegex == "exact" {
				pathMatchType = gatewayv1.PathMatchExact
			} else {
				// "true", "case_sensitive", "case_insensitive" all use regex
				pathMatchType = gatewayv1.PathMatchRegularExpression

				// Add a general warning about NGF not supporting regex
				message := "nginx.org/path-regex: PathMatchRegularExpression is not supported by NGINX Gateway Fabric - only Exact and PathPrefix are supported"
				notify(notifications.WarningNotification, message, &rule.Ingress)

				// Add a warning for case_insensitive since Gateway API doesn't guarantee it
				if pathRegex == "case_insensitive" {
					message := "nginx.org/path-regex: case_insensitive - injected (?i) regex flag but case insensitive behavior depends on Gateway implementation support"
					notify(notifications.WarningNotification, message, &rule.Ingress)
				}
			}

			// Get the HTTPRoute for this rule group
			key := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRouteContext, ok := ir.HTTPRoutes[key]
			if !ok {
				continue
			}

			for _, routeRule := range httpRouteContext.HTTPRoute.Spec.Rules {
				for _, match := range routeRule.Matches {
					if match.Path != nil {
						match.Path.Type = ptr.To(pathMatchType)

						// For case_insensitive regex, inject (?i) flag at the beginning
						if pathRegex == "case_insensitive" && pathMatchType == gatewayv1.PathMatchRegularExpression {
							if match.Path.Value != nil {
								originalPath := *match.Path.Value
								// Only inject if not already present
								if !strings.HasPrefix(originalPath, "(?i)") {
									caseInsensitivePath := "(?i)" + originalPath
									match.Path.Value = &caseInsensitivePath
								}
							}
						}
					}
				}
			}

			// Update the HTTPRoute in the IR
			ir.HTTPRoutes[key] = httpRouteContext
		}
	}

	return errs
}
