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
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// HeaderManipulationFeature converts header manipulation annotations to HTTPRoute filters
func HeaderManipulationFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			// Get the HTTPRoute for this rule group
			key := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRouteContext, ok := ir.HTTPRoutes[key]
			if !ok {
				return field.ErrorList{field.InternalError(nil, fmt.Errorf("HTTPRoute does not exist - common HTTPRoute generation failed"))}
			}

			// Process proxy-hide-headers annotation
			if hideHeaders, exists := rule.Ingress.Annotations[nginxProxyHideHeadersAnnotation]; exists && hideHeaders != "" {
				filter := createResponseHeaderModifier(hideHeaders)
				if filter != nil {
					errs = append(errs, addFilterToHTTPRoute(&httpRouteContext.HTTPRoute, rule.Ingress, *filter)...)
				}
			}

			// Process proxy-set-headers annotation
			if setHeaders, exists := rule.Ingress.Annotations[nginxProxySetHeadersAnnotation]; exists && setHeaders != "" {
				filter := createRequestHeaderModifier(setHeaders)
				if filter != nil {
					errs = append(errs, addFilterToHTTPRoute(&httpRouteContext.HTTPRoute, rule.Ingress, *filter)...)
				}
			}

			// Update the HTTPRoute in the IR
			ir.HTTPRoutes[key] = httpRouteContext
		}
	}

	return errs
}

// addFilterToHTTPRoute adds a filter to all HTTPRoute rules
//
//nolint:unparam // ErrorList return type maintained for consistency
func addFilterToHTTPRoute(httpRoute *gatewayv1.HTTPRoute, _ networkingv1.Ingress, filter gatewayv1.HTTPRouteFilter) field.ErrorList {
	var errs field.ErrorList

	// Apply filter to all rules
	for i := range httpRoute.Spec.Rules {
		if httpRoute.Spec.Rules[i].Filters == nil {
			httpRoute.Spec.Rules[i].Filters = []gatewayv1.HTTPRouteFilter{}
		}
		httpRoute.Spec.Rules[i].Filters = append(httpRoute.Spec.Rules[i].Filters, filter)
	}

	return errs
}

// createResponseHeaderModifier creates a ResponseHeaderModifier filter from comma-separated header names
func createResponseHeaderModifier(hideHeaders string) *gatewayv1.HTTPRouteFilter {
	headersToRemove := parseCommaSeparatedHeaders(hideHeaders)
	if len(headersToRemove) == 0 {
		return nil
	}

	return &gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
		ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
			Remove: headersToRemove,
		},
	}
}

// createRequestHeaderModifier creates a RequestHeaderModifier filter from proxy-set-headers annotation
func createRequestHeaderModifier(setHeaders string) *gatewayv1.HTTPRouteFilter {
	headers := parseSetHeaders(setHeaders)
	if len(headers) == 0 {
		return nil
	}

	var headersToSet []gatewayv1.HTTPHeader
	for name, value := range headers {
		if value != "" && !strings.Contains(value, "$") {
			headersToSet = append(headersToSet, gatewayv1.HTTPHeader{
				Name:  gatewayv1.HTTPHeaderName(name),
				Value: value,
			})
		}
		// Note: Headers with NGINX variables cannot be converted to Gateway API
		// as Gateway API doesn't support dynamic header values
	}

	if len(headersToSet) == 0 {
		return nil
	}

	return &gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
			Set: headersToSet,
		},
	}
}

// parseCommaSeparatedHeaders parses a comma-separated list of header names
func parseCommaSeparatedHeaders(headersList string) []string {
	return splitAndTrimCommaList(headersList)
}

// parseSetHeaders parses nginx.org/proxy-set-headers annotation format
// Supports both header names and header:value pairs
func parseSetHeaders(setHeaders string) map[string]string {
	headers := make(map[string]string)
	parts := splitAndTrimCommaList(setHeaders)

	for _, part := range parts {
		if strings.Contains(part, ":") {
			// Format: "Header-Name: value"
			kv := strings.SplitN(part, ":", 2)
			if len(kv) == 2 {
				headerName := strings.TrimSpace(kv[0])
				headerValue := strings.TrimSpace(kv[1])
				if headerName != "" {
					headers[headerName] = headerValue
				}
			}
		}
		// Note: Headers without explicit values (format "$Variable-Name") are skipped
		// as Gateway API cannot use NGINX variables like $http_* and headers need values
	}

	return headers
}

// Note: The patchHTTPRouteHeaderMatching function has been removed as it was incomplete.
// Header matching should be implemented separately if needed for specific NGINX features.
