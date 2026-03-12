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
	"strconv"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// HSTSFeature converts HSTS annotations to HTTPRoute ResponseHeaderModifier filters.
// Supports nginx.org/hsts, nginx.org/hsts-max-age, and nginx.org/hsts-include-subdomains annotations.
func HSTSFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			if hsts, ok := rule.Ingress.Annotations[nginxHSTSAnnotation]; ok && hsts == "true" {
				errs = append(errs, processHSTSAnnotation(notify, rule.Ingress, ir)...)
			}
		}
	}

	return errs
}

//nolint:unparam // ErrorList return type maintained for consistency
func processHSTSAnnotation(notify notifications.NotifyFunc, ingress networkingv1.Ingress, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	hstsHeader := "Strict-Transport-Security"
	hstsMaxAge := "31536000" // Default max-age of 1 year
	hstsIncludeSubdomain := false

	// Parse the HSTS max-age value
	if maxAge, ok := ingress.Annotations[nginxHSTSMaxAgeAnnotation]; ok && maxAge != "" {
		if _, err := strconv.Atoi(maxAge); err != nil {
			notify(notifications.ErrorNotification, "nginx.org/hsts-max-age: Invalid max-age value, must be a number", &ingress)
			// Continue with default value instead of failing
		} else {
			hstsMaxAge = maxAge
		}
	}

	if includeSubdomains, ok := ingress.Annotations[nginxHSTSIncludeSubdomainsAnnotation]; ok && includeSubdomains == "true" {
		hstsIncludeSubdomain = true
	}

	hstsHeaderValue := buildHSTS(hstsMaxAge, hstsIncludeSubdomain)

	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		routeName := common.RouteName(ingress.Name, rule.Host)
		routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

		httpRouteContext, exists := ir.HTTPRoutes[routeKey]
		if !exists {
			continue
		}

		for i := range httpRouteContext.HTTPRoute.Spec.Rules {
			if httpRouteContext.HTTPRoute.Spec.Rules[i].Filters == nil {
				httpRouteContext.HTTPRoute.Spec.Rules[i].Filters = []gatewayv1.HTTPRouteFilter{}
			}

			filter := gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{
							Name:  gatewayv1.HTTPHeaderName(hstsHeader),
							Value: hstsHeaderValue,
						},
					},
				},
			}

			httpRouteContext.HTTPRoute.Spec.Rules[i].Filters = append(httpRouteContext.HTTPRoute.Spec.Rules[i].Filters, filter)
		}

		// Update the route context in the IR
		ir.HTTPRoutes[routeKey] = httpRouteContext
	}

	return errs
}

func buildHSTS(hstsMaxAge string, hstsIncludeSubdomain bool) string {
	parts := []string{
		"max-age=" + hstsMaxAge,
	}
	if hstsIncludeSubdomain {
		parts = append(parts, "includeSubDomains")
	}
	return strings.Join(parts, "; ")
}
