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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// redirectFeature parses redirect annotations (permanent-redirect, temporal-redirect)
// and applies them to HTTPRoute rules.
func redirectFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	for _, httpRouteContext := range ir.HTTPRoutes {
		for i := range httpRouteContext.HTTPRoute.Spec.Rules {
			if i >= len(httpRouteContext.RuleBackendSources) {
				continue
			}
			sources := httpRouteContext.RuleBackendSources[i]

			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			redirectFilter, parseErrs := parseRedirectAnnotations(ingress, &httpRouteContext.HTTPRoute)
			errs = append(errs, parseErrs...)

			if redirectFilter != nil {
				// Add the redirect filter to the rule
				httpRouteContext.HTTPRoute.Spec.Rules[i].Filters = append(
					httpRouteContext.HTTPRoute.Spec.Rules[i].Filters,
					*redirectFilter,
				)
				notify(notifications.InfoNotification, fmt.Sprintf("Applied redirect to rule %d of route %s/%s", i, httpRouteContext.HTTPRoute.Namespace, httpRouteContext.HTTPRoute.Name), &httpRouteContext.HTTPRoute)
			}

			// Warn about unsupported redirect annotations
			warnUnsupportedRedirectAnnotations(ingress, &httpRouteContext.HTTPRoute)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// parseRedirectAnnotations parses permanent-redirect and temporal-redirect annotations
func parseRedirectAnnotations(ingress *networkingv1.Ingress, httpRoute *gatewayv1.HTTPRoute) (*gatewayv1.HTTPRouteFilter, field.ErrorList) {
	var errs field.ErrorList

	// Check for permanent-redirect first (takes precedence)
	if redirectURL, ok := ingress.Annotations[PermanentRedirectAnnotation]; ok && redirectURL != "" {
		statusCode := 301 // Default for permanent redirect

		// Check for custom status code
		if codeStr, ok := ingress.Annotations[PermanentRedirectCodeAnnotation]; ok && codeStr != "" {
			code, err := strconv.Atoi(codeStr)
			if err != nil {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", PermanentRedirectCodeAnnotation),
					codeStr,
					fmt.Sprintf("invalid redirect code: %v", err),
				))
			} else if code < 300 || code > 399 {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", PermanentRedirectCodeAnnotation),
					codeStr,
					"redirect code must be between 300 and 399",
				))
			} else {
				statusCode = code
			}
		}

		// Warn if query string or fragment will be ignored
		if hasQueryOrFragment(redirectURL) {
			notify(notifications.WarningNotification, fmt.Sprintf("Ingress %s/%s: query string and fragment in redirect URL will be ignored (Gateway API limitation)", ingress.Namespace, ingress.Name), httpRoute)
		}

		filter, parseErr := createRedirectFilter(redirectURL, statusCode)
		if parseErr != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", PermanentRedirectAnnotation),
				redirectURL,
				fmt.Sprintf("invalid redirect URL: %v", parseErr),
			))
			return nil, errs
		}

		notify(notifications.InfoNotification, fmt.Sprintf("Ingress %s/%s: parsed permanent-redirect=%s with code %d", ingress.Namespace, ingress.Name, redirectURL, statusCode), httpRoute)
		return filter, errs
	}

	// Check for temporal-redirect
	if redirectURL, ok := ingress.Annotations[TemporalRedirectAnnotation]; ok && redirectURL != "" {
		statusCode := 302 // Default for temporal redirect

		// Warn if query string or fragment will be ignored
		if hasQueryOrFragment(redirectURL) {
			notify(notifications.WarningNotification, fmt.Sprintf("Ingress %s/%s: query string and fragment in redirect URL will be ignored (Gateway API limitation)", ingress.Namespace, ingress.Name), httpRoute)
		}

		filter, parseErr := createRedirectFilter(redirectURL, statusCode)
		if parseErr != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", TemporalRedirectAnnotation),
				redirectURL,
				fmt.Sprintf("invalid redirect URL: %v", parseErr),
			))
			return nil, errs
		}

		notify(notifications.InfoNotification, fmt.Sprintf("Ingress %s/%s: parsed temporal-redirect=%s with code %d", ingress.Namespace, ingress.Name, redirectURL, statusCode), httpRoute)
		return filter, errs
	}

	return nil, errs
}

// createRedirectFilter creates an HTTPRouteFilter for a redirect URL
func createRedirectFilter(redirectURL string, statusCode int) (*gatewayv1.HTTPRouteFilter, error) {
	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		return nil, err
	}

	filter := &gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
			StatusCode: ptr.To(statusCode),
		},
	}

	// Set scheme if present
	if parsedURL.Scheme != "" {
		filter.RequestRedirect.Scheme = ptr.To(parsedURL.Scheme)
	}

	// Set hostname if present
	if parsedURL.Hostname() != "" {
		hostname := gatewayv1.PreciseHostname(parsedURL.Hostname())
		filter.RequestRedirect.Hostname = &hostname
	}

	// Set port if present
	if parsedURL.Port() != "" {
		port, err := strconv.Atoi(parsedURL.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %v", err)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port must be between 1 and 65535, got %d", port)
		}
		portNum := gatewayv1.PortNumber(port)
		filter.RequestRedirect.Port = &portNum
	}

	// Set path if present (excluding root path which is the default)
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		filter.RequestRedirect.Path = &gatewayv1.HTTPPathModifier{
			Type:            gatewayv1.FullPathHTTPPathModifier,
			ReplaceFullPath: ptr.To(parsedURL.Path),
		}
	}

	return filter, nil
}

// hasQueryOrFragment checks if a URL contains query string or fragment
func hasQueryOrFragment(redirectURL string) bool {
	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}
	return parsedURL.RawQuery != "" || parsedURL.Fragment != ""
}

// warnUnsupportedRedirectAnnotations logs warnings for redirect annotations that
// cannot be directly translated to Gateway API
func warnUnsupportedRedirectAnnotations(ingress *networkingv1.Ingress, httpRoute *gatewayv1.HTTPRoute) {
	if _, ok := ingress.Annotations[FromToWWWRedirectAnnotation]; ok {
		notify(notifications.WarningNotification, fmt.Sprintf("Ingress %s/%s: from-to-www-redirect is not directly supported in Gateway API (requires multiple routes)", ingress.Namespace, ingress.Name), httpRoute)
	}
}

// Ingress NGINX has some quirky behaviors around SSL redirect.
// The formula we follow is that if an ingress has certs configured, and it does not have the
// "nginx.ingress.kubernetes.io/ssl-redirect" annotation set to "false" (or "0", etc), then we
// enable SSL redirect for that host.
// Also supports force-ssl-redirect which enables SSL redirect even without TLS configuration.
func addDefaultSSLRedirect(pir *providerir.ProviderIR, eir *emitterir.EmitterIR) field.ErrorList {
	for key, httpRouteContext := range pir.HTTPRoutes {
		hasSecrets := false
		enableRedirect := true
		forceRedirect := false

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
						fmt.Sprintf("failed to parse ssl-redirect annotation: %v", err),
					)}
				}
				enableRedirect = parsed
			}

			// Check the force-ssl-redirect annotation.
			if val, ok := ingress.Annotations[ForceSSLRedirectAnnotation]; ok {
				parsed, err := strconv.ParseBool(val)
				if err != nil {
					return field.ErrorList{field.Invalid(
						field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations"),
						ingress.Annotations,
						fmt.Sprintf("failed to parse force-ssl-redirect annotation: %v", err),
					)}
				}
				forceRedirect = parsed
			}
		}

		// Enable SSL redirect if:
		// 1. Has TLS secrets and ssl-redirect is not disabled, OR
		// 2. force-ssl-redirect is true
		if !((hasSecrets && enableRedirect) || forceRedirect) {
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
