/*
Copyright 2026 The Kubernetes Authors.

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

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Iterate over all HTTPRoutes in the IR
	for key, httpRouteContext := range ir.HTTPRoutes {
		// Iterate over each rule in the HTTPRoute
		for ruleIndex := range httpRouteContext.HTTPRoute.Spec.Rules {
			// Check if this rule has backend sources
			if ruleIndex >= len(httpRouteContext.RuleBackendSources) {
				continue
			}

			// Get the non canary ingress for this rule
			ingress := getNonCanaryIngress(httpRouteContext.RuleBackendSources[ruleIndex])

			if ingress == nil {
				continue
			}

			// Warn about unsupported proxy-redirect annotations
			if ingress.Annotations[ProxyRedirectFromAnnotation] != "" {
				notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
					ingress.Namespace, ingress.Name, ProxyRedirectFromAnnotation), ingress)
			}
			if ingress.Annotations[ProxyRedirectToAnnotation] != "" {
				notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
					ingress.Namespace, ingress.Name, ProxyRedirectToAnnotation), ingress)
			}

			permanentRedirectURL, hasPermanent := ingress.Annotations[PermanentRedirectAnnotation]
			temporalRedirectURL, hasTemporal := ingress.Annotations[TemporalRedirectAnnotation]

			// Skip if neither annotation is present
			if !hasPermanent && !hasTemporal {
				continue
			}

			// Determine redirect URL and status code
			// If both are present, temporal takes priority
			var redirectURL string
			var statusCode int
			var annotationUsed string

			if hasTemporal {
				redirectURL = temporalRedirectURL
				statusCode = 302
				annotationUsed = TemporalRedirectAnnotation

				// Warn if both annotations are present
				if hasPermanent {
					notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s has both %s and %s annotations, using %s",
						ingress.Namespace, ingress.Name, PermanentRedirectAnnotation, TemporalRedirectAnnotation, TemporalRedirectAnnotation), ingress)
				}

				// Check custom status code annotation
				if codeStr := ingress.Annotations[TemporalRedirectCodeAnnotation]; codeStr != "" {
					code, err := strconv.Atoi(codeStr)
					if err != nil || code != 302 {
						notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported status code %q in %s annotation, using default 302",
							ingress.Namespace, ingress.Name, codeStr, TemporalRedirectCodeAnnotation), ingress)
					}
					// If code is 302, we allow it (it matches the default, so no change needed)
				}
			} else {
				redirectURL = permanentRedirectURL
				statusCode = 301
				annotationUsed = PermanentRedirectAnnotation

				// Check custom status code annotation
				if codeStr := ingress.Annotations[PermanentRedirectCodeAnnotation]; codeStr != "" {
					code, err := strconv.Atoi(codeStr)
					if err != nil || code != 301 {
						notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported status code %q in %s annotation, using default 301",
							ingress.Namespace, ingress.Name, codeStr, PermanentRedirectCodeAnnotation), ingress)
					}
					// If code is 301, we allow it (it matches the default, so no change needed)
				}
			}

			// Validate that the redirect URL is not empty
			if redirectURL == "" {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", annotationUsed),
					redirectURL,
					"redirect URL cannot be empty",
				))
				continue
			}

			// Parse the redirect URL
			parsedURL, err := url.Parse(redirectURL)
			if err != nil {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", annotationUsed),
					redirectURL,
					fmt.Sprintf("invalid redirect URL: %v", err),
				))
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
				} else {
					errs = append(errs, field.Invalid(
						field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", annotationUsed),
						redirectURL,
						fmt.Sprintf("invalid port in redirect URL: %v", err),
					))
					continue
				}
			}

			// Set path - default to root path if not specified in redirect URL
			// This matches ingress-nginx behavior where redirects override the request path
			path := parsedURL.Path
			if path == "" {
				path = "/"
			}
			pathType := gatewayv1.FullPathHTTPPathModifier
			redirectFilterConfig.Path = &gatewayv1.HTTPPathModifier{
				Type:            pathType,
				ReplaceFullPath: ptr.To(path),
			}

			redirectFilter := gatewayv1.HTTPRouteFilter{
				Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
				RequestRedirect: redirectFilterConfig,
			}

			// Add redirect filter to the current rule
			httpRouteContext.HTTPRoute.Spec.Rules[ruleIndex].Filters = append(
				httpRouteContext.HTTPRoute.Spec.Rules[ruleIndex].Filters,
				redirectFilter,
			)

			// Clear backend refs as redirects don't route to backends
			httpRouteContext.HTTPRoute.Spec.Rules[ruleIndex].BackendRefs = nil

			notify(notifications.InfoNotification,
				fmt.Sprintf("parsed %q annotation of ingress %s/%s with redirect to %q (status code: %d). ",
					annotationUsed, ingress.Namespace, ingress.Name, redirectURL, statusCode),
				&httpRouteContext.HTTPRoute)
		}

		// Save the updated context back to the IR
		ir.HTTPRoutes[key] = httpRouteContext
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

		if !hasSecrets || !enableRedirect {
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
