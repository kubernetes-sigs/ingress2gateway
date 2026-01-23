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
	"net/url"
	"strconv"

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

			// Warn about unsupported proxy-redirect annotations
			if rule.Ingress.Annotations[ProxyRedirectFromAnnotation] != "" {
				notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
					rule.Ingress.Namespace, rule.Ingress.Name, ProxyRedirectFromAnnotation), &rule.Ingress)
			}
			if rule.Ingress.Annotations[ProxyRedirectToAnnotation] != "" {
				notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
					rule.Ingress.Namespace, rule.Ingress.Name, ProxyRedirectToAnnotation), &rule.Ingress)
			}

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

				// Warn about unsupported custom status code annotation
				if rule.Ingress.Annotations[PermanentRedirectCodeAnnotation] != "" {
					notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
						rule.Ingress.Namespace, rule.Ingress.Name, PermanentRedirectCodeAnnotation), &rule.Ingress)
				}
			} else {
				redirectURL = temporalRedirectURL
				statusCode = 302
				annotationUsed = TemporalRedirectAnnotation

				// Warn about unsupported custom status code annotation
				if rule.Ingress.Annotations[TemporalRedirectCodeAnnotation] != "" {
					notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
						rule.Ingress.Namespace, rule.Ingress.Name, TemporalRedirectCodeAnnotation), &rule.Ingress)
				}
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

			// Parse the redirect URL
			parsedURL, err := url.Parse(redirectURL)
			if err != nil {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", rule.Ingress.Namespace, rule.Ingress.Name, "metadata", "annotations", annotationUsed),
					redirectURL,
					fmt.Sprintf("invalid redirect URL: %v", err),
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
				redirectFilterConfig := &gatewayv1.HTTPRequestRedirectFilter{
					StatusCode: ptr.To(statusCode),
				}

				// Set scheme if present
				if parsedURL.Scheme != "" {
					redirectFilterConfig.Scheme = ptr.To(parsedURL.Scheme)
				}

				// Set hostname if present
				if parsedURL.Hostname() != "" {
					hostname := gatewayv1.PreciseHostname(parsedURL.Hostname())
					redirectFilterConfig.Hostname = &hostname
				}

				// Set port if present
				if parsedURL.Port() != "" {
					port, err := strconv.Atoi(parsedURL.Port())
					if err == nil {
						portNumber := gatewayv1.PortNumber(port)
						redirectFilterConfig.Port = &portNumber
					}
				}

				// Set path if present
				if parsedURL.Path != "" {
					pathType := gatewayv1.FullPathHTTPPathModifier
					redirectFilterConfig.Path = &gatewayv1.HTTPPathModifier{
						Type:            pathType,
						ReplaceFullPath: ptr.To(parsedURL.Path),
					}
				}

				redirectFilter := gatewayv1.HTTPRouteFilter{
					Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
					RequestRedirect: redirectFilterConfig,
				}

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
					fmt.Sprintf("parsed %q annotation of ingress %s/%s with redirect to %q (status code: %d). ",
						annotationUsed, rule.Ingress.Namespace, rule.Ingress.Name, redirectURL, statusCode),
					&httpRouteContext.HTTPRoute)
			}
		}
	}
  
	return errs
}

// Ingress NGINX has some quirky behaviors around SSL redirect.
// The formula we follow is that if an ingress has certs configured, and it does not have the
// "nginx.ingress.kubernetes.io/ssl-redirect" annotation set to "false" (or "0", etc), then we
// enable SSL redirect for that host.
func addDefaultSSLRedirect(pir *providerir.ProviderIR, eir *emitterir.EmitterIR) field.ErrorList {
	for key, httpRouteContext := range pir.HTTPRoutes {
		hasSecrets := false
		enableRedirect := true

		for _, sources := range httpRouteContext.RuleBackendSources {
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			// Check if the ingress has TLS secrets.
			if len(ingress.Spec.TLS) > 0 {
				hasSecrets = true
			}

			// Check the ssl-redirect annotation.
			if val, ok := ingress.Annotations[SSLRedirectAnnotation]; ok {
				parsed, err := strconv.ParseBool(val)
				if err != nil {
					return field.ErrorList{field.Invalid(
						field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations"),
						ingress.Annotations,
						fmt.Sprintf("failed to parse canary configuration: %v", err),
					)}
				}
				enableRedirect = parsed
			}
		}

		if !(hasSecrets && enableRedirect) {
			continue
		}

		redirectRoute := gatewayv1.HTTPRoute{
			TypeMeta: httpRouteContext.HTTPRoute.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-ssl-redirect", httpRouteContext.HTTPRoute.Name),
				Namespace: httpRouteContext.HTTPRoute.Namespace,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				Hostnames: httpRouteContext.HTTPRoute.Spec.DeepCopy().Hostnames,
				Rules: []gatewayv1.HTTPRouteRule{
					{
						Filters: []gatewayv1.HTTPRouteFilter{
							{
								Type: gatewayv1.HTTPRouteFilterRequestRedirect,
								RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
									Scheme:     ptr.To("https"),
									StatusCode: ptr.To(308),
								},
							},
						},
					},
				},
			},
		}
		// add parentrefs
		redirectRoute.Spec.ParentRefs = httpRouteContext.HTTPRoute.Spec.DeepCopy().ParentRefs
		// bind to port 80
		for i := range redirectRoute.Spec.ParentRefs {
			redirectRoute.Spec.ParentRefs[i].Port = ptr.To[int32](80)
		}
		eir.HTTPRoutes[types.NamespacedName{
			Namespace: redirectRoute.Namespace,
			Name:      redirectRoute.Name,
		}] = emitterir.HTTPRouteContext{
			HTTPRoute: redirectRoute,
		}
		// bind this to port 443
		eHTTPRouteContext := eir.HTTPRoutes[key]
		for i := range eHTTPRouteContext.Spec.ParentRefs {
			eHTTPRouteContext.Spec.ParentRefs[i].Port = ptr.To[int32](443)
		}
		eir.HTTPRoutes[key] = eHTTPRouteContext
	}
	return nil
}
