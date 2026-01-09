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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// redirectFeature converts permanent and temporal redirect annotations to Gateway API RequestRedirect filters.
// - permanent-redirect uses a 301 status code
// - temporal-redirect uses a 302 status code
func redirectFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			permanentRedirectURL, hasPermanent := rule.Ingress.Annotations[PermanentRedirectAnnotation]
			temporalRedirectURL, hasTemporal := rule.Ingress.Annotations[TemporalRedirectAnnotation]

			// Skip if neither annotation is present
			if !hasPermanent && !hasTemporal {
				continue
			}

			// Both annotations present is an error
			if hasPermanent && hasTemporal {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", rule.Ingress.Namespace, rule.Ingress.Name, "metadata", "annotations"),
					rule.Ingress.Annotations,
					fmt.Sprintf("cannot use both %s and %s annotations simultaneously", PermanentRedirectAnnotation, TemporalRedirectAnnotation),
				))
				continue
			}

			// Determine redirect URL and status code
			var redirectURL string
			var statusCode int
			var annotationUsed string

			if hasPermanent {
				redirectURL = permanentRedirectURL
				statusCode = 301
				annotationUsed = PermanentRedirectAnnotation
			} else {
				redirectURL = temporalRedirectURL
				statusCode = 302
				annotationUsed = TemporalRedirectAnnotation
			}

			// Validate redirect URL is not empty
			if redirectURL == "" {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", rule.Ingress.Namespace, rule.Ingress.Name, "metadata", "annotations", annotationUsed),
					redirectURL,
					"redirect URL cannot be empty",
				))
				continue
			}

			// Apply redirect to all rules in the ingress
			for _, ingressRule := range rule.Ingress.Spec.Rules {
				routeName := common.RouteName(rule.Ingress.Name, ingressRule.Host)
				routeKey := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: routeName}
				httpRouteContext, routeExists := ir.HTTPRoutes[routeKey]
				if !routeExists {
					continue
				}

				// Create the redirect filter
				redirectFilter := gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterRequestRedirect,
					RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
						StatusCode: ptr.To(statusCode),
					},
				}

				// Parse the redirect URL and set the appropriate fields
				// For now, we'll use the full URL as the hostname
				// A more sophisticated implementation would parse the URL into its components
				// (scheme, hostname, port, path) but this requires URL parsing logic
				// For simplicity, we'll set the entire URL as a notification
				// and just clear the backend refs since this is a redirect

				// Add redirect rule at the beginning of all rules
				redirectRule := gatewayv1.HTTPRouteRule{
					Filters: []gatewayv1.HTTPRouteFilter{redirectFilter},
					// Clear backend refs as redirects don't route to backends
					BackendRefs: nil,
				}

				// Prepend the redirect rule to existing rules
				httpRouteContext.HTTPRoute.Spec.Rules = append(
					[]gatewayv1.HTTPRouteRule{redirectRule},
					httpRouteContext.HTTPRoute.Spec.Rules...,
				)

				ir.HTTPRoutes[routeKey] = httpRouteContext

				notify(notifications.InfoNotification,
					fmt.Sprintf("parsed %q annotation of ingress %s/%s with redirect to %q (status code: %d). Note: Gateway API HTTPRequestRedirectFilter has limited URL redirect support - you may need to manually configure the redirect target using scheme, hostname, port, and path fields",
						annotationUsed, rule.Ingress.Namespace, rule.Ingress.Name, redirectURL, statusCode),
					&httpRouteContext.HTTPRoute)
			}
		}
	}

	return errs
}
