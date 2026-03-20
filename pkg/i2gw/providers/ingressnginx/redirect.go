/*
Copyright The Kubernetes Authors.

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
	"strings"

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
// This matches ingress-nginx's execution order: temporal redirect is checked first, then permanent.
// If the temporal-redirect annotation key is present (even with an empty value), the function
// short-circuits and permanent redirect annotations are never evaluated.
//
// Gateway API only supports status codes 301, 302, 303, 307, 308.
// Intersecting with ingress-nginx's valid ranges:
// - temporal-redirect defaults to 302, supported custom codes: 301, 302, 303, 307
// - permanent-redirect defaults to 301, supported custom codes: 301, 302, 303, 307, 308.
func redirectFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {

	// Iterate over all HTTPRoutes in the IR.
	for key, httpRouteContext := range ir.HTTPRoutes {
		// Iterate over each rule in the HTTPRoute.
		for ruleIndex := range httpRouteContext.HTTPRoute.Spec.Rules {
			// Check if this rule has backend sources.
			if ruleIndex >= len(httpRouteContext.RuleBackendSources) {
				continue
			}

			// Get the non canary ingress for this rule.
			ingress := getNonCanaryIngress(httpRouteContext.RuleBackendSources[ruleIndex])

			if ingress == nil {
				continue
			}

			// Warn about unsupported proxy-redirect annotations.
			if ingress.Annotations[ProxyRedirectFromAnnotation] != "" {
				notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
					ingress.Namespace, ingress.Name, ProxyRedirectFromAnnotation), ingress)
			}
			if ingress.Annotations[ProxyRedirectToAnnotation] != "" {
				notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
					ingress.Namespace, ingress.Name, ProxyRedirectToAnnotation), ingress)
			}

			temporalRedirectURL, hasTemporal := ingress.Annotations[TemporalRedirectAnnotation]
			permanentRedirectURL, hasPermanent := ingress.Annotations[PermanentRedirectAnnotation]

			// Skip if neither annotation is present.
			if !hasPermanent && !hasTemporal {
				continue
			}

			// Determine redirect URL and status code.
			// Matching ingress-nginx execution order: temporal is checked first.
			// If the temporal-redirect annotation key is present (even with an empty value),
			// the function short-circuits — permanent redirect is never evaluated.
			var redirectURL string
			var statusCode int
			var annotationUsed string

			if hasTemporal {
				redirectURL = temporalRedirectURL
				statusCode = 302
				annotationUsed = TemporalRedirectAnnotation

				// Warn if both annotations are present (permanent is ignored).
				if hasPermanent {
					notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s has both %s and %s annotations; temporal-redirect takes priority, permanent-redirect is ignored",
						ingress.Namespace, ingress.Name, PermanentRedirectAnnotation, TemporalRedirectAnnotation), ingress)
				}

				// Check custom status code annotation.
				if codeStr := ingress.Annotations[TemporalRedirectCodeAnnotation]; codeStr != "" {
					code, err := strconv.Atoi(codeStr)
					if err != nil || !isValidTemporalRedirectCode(code) {
						notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported status code %q in %s annotation (Gateway API supports: 301, 302, 303, 307 for temporal redirects), using default 302",
							ingress.Namespace, ingress.Name, codeStr, TemporalRedirectCodeAnnotation), ingress)
					} else {
						statusCode = code
					}
				}
			} else {
				// Only reached if temporal-redirect annotation is completely absent.
				redirectURL = permanentRedirectURL
				statusCode = 301
				annotationUsed = PermanentRedirectAnnotation

				// Check custom status code annotation.
				if codeStr := ingress.Annotations[PermanentRedirectCodeAnnotation]; codeStr != "" {
					code, err := strconv.Atoi(codeStr)
					if err != nil || !isValidPermanentRedirectCode(code) {
						notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported status code %q in %s annotation (Gateway API supports: 301, 302, 303, 307, 308 for permanent redirects), using default 301",
							ingress.Namespace, ingress.Name, codeStr, PermanentRedirectCodeAnnotation), ingress)
					} else {
						statusCode = code
					}
				}
			}

			// Validate that the redirect URL is not empty.
			if redirectURL == "" {
				notify(notifications.ErrorNotification, fmt.Sprintf("Empty %s annotation, skipping redirect",
					annotationUsed), ingress)
				continue
			}

			// Parse the redirect URL.
			parsedURL, err := url.Parse(redirectURL)
			if err != nil {
				notify(notifications.ErrorNotification, fmt.Sprintf("Invalid redirect URL in %s annotation: %v, skipping redirect",
					annotationUsed, err), ingress)
				continue
			}

			// Create the redirect filter.
			redirectFilterConfig := &gatewayv1.HTTPRequestRedirectFilter{
				StatusCode: ptr.To(statusCode),
			}

			// Set scheme if present.
			if parsedURL.Scheme != "" {
				redirectFilterConfig.Scheme = ptr.To(parsedURL.Scheme)
			}

			// Set hostname if present.
			if parsedURL.Hostname() != "" {
				hostname := gatewayv1.PreciseHostname(parsedURL.Hostname())
				redirectFilterConfig.Hostname = &hostname
			}

			// Set port if present.
			if parsedURL.Port() != "" {
				port, err := strconv.Atoi(parsedURL.Port())
				if err == nil {
					portNumber := gatewayv1.PortNumber(port)
					redirectFilterConfig.Port = &portNumber
				} else {
					notify(notifications.ErrorNotification, fmt.Sprintf("Invalid port in redirect URL %q: %v, skipping redirect",
						redirectURL, err), ingress)
					continue
				}
			}

			// Set path - default to root path if not specified in redirect URL
			// This matches ingress-nginx behavior where redirects override the request path.
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

			// Add redirect filter to the current rule.
			httpRouteContext.HTTPRoute.Spec.Rules[ruleIndex].Filters = append(
				httpRouteContext.HTTPRoute.Spec.Rules[ruleIndex].Filters,
				redirectFilter,
			)

			// Clear backend refs as redirects don't route to backends.
			httpRouteContext.HTTPRoute.Spec.Rules[ruleIndex].BackendRefs = nil
		}

		// Save the updated context back to the IR.
		ir.HTTPRoutes[key] = httpRouteContext
	}

	return nil
}

// addSSLAndTrailingSlashRedirects adds HTTP→HTTPS redirect routes and trailing slash
// redirect rules to match ingress-nginx behavior.
//
// SSL redirect: In ingress-nginx, TLS is merged at the hostname/server level — if any
// ingress on a hostname has TLS configured, the cert applies to the entire server block
// and ssl-redirect defaults to true for all locations. A rule only skips the redirect if
// its source ingress explicitly sets "nginx.ingress.kubernetes.io/ssl-redirect" to "false".
//
// Trailing slash redirect: NGINX implicitly redirects /path to /path/ with 301 when a
// location /path/ {} block exists. On the HTTP (port 80) side, trailing slash redirects
// are combined with the SSL upgrade into a single hop (301 to https://host/path/) to
// avoid unnecessary intermediate redirects.
func (p *Provider) addSSLAndTrailingSlashRedirects(ingresses []networkingv1.Ingress, pir *providerir.ProviderIR, eir *emitterir.EmitterIR) {
	// Find hosts with TLS enabled.
	hostsWithTLS := make(map[string]struct{})
	for _, ing := range ingresses {
		for _, tls := range ing.Spec.TLS {
			if len(tls.Hosts) > 0 {
				for _, host := range tls.Hosts {
					hostsWithTLS[host] = struct{}{}
				}
			} else {
				for _, rule := range ing.Spec.Rules {
					if rule.Host != "" {
						hostsWithTLS[rule.Host] = struct{}{}
					}
				}
				break
			}
		}
	}

	for key, httpRouteContext := range pir.HTTPRoutes {
		eRouteCtx, ok := eir.HTTPRoutes[key]
		if !ok {
			continue
		}

		// Compute trailing slash redirect rules for this route.
		trailingSlashRules := buildTrailingSlashRedirectRules(eRouteCtx.Spec.Rules)

		// Add trailing slash rules to the main route (HTTPS side, or the only route if no TLS).
		eRouteCtx.Spec.Rules = append(eRouteCtx.Spec.Rules, trailingSlashRules...)

		hostHasTLS := false
		for _, hostname := range httpRouteContext.HTTPRoute.Spec.Hostnames {
			if _, ok := hostsWithTLS[string(hostname)]; ok {
				hostHasTLS = true
				break
			}
		}

		if !hostHasTLS {
			eir.HTTPRoutes[key] = eRouteCtx
			continue
		}

		// Build HTTP (port 80) rules: per-rule SSL redirect or pass-through.
		var httpRules []gatewayv1.HTTPRouteRule
		hasRedirect := false
		for ruleIdx, sources := range httpRouteContext.RuleBackendSources {
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			if ruleIdx >= len(eRouteCtx.Spec.Rules) {
				continue
			}

			enableRedirect := true
			if val, ok := ingress.Annotations[SSLRedirectAnnotation]; ok {
				enableRedirect, _ = strconv.ParseBool(val)
			}

			if enableRedirect {
				hasRedirect = true
				rule := gatewayv1.HTTPRouteRule{
					Matches: eRouteCtx.Spec.Rules[ruleIdx].DeepCopy().Matches,
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
								Scheme:     ptr.To("https"),
								StatusCode: ptr.To(308),
							},
						},
					},
				}
				httpRules = append(httpRules, rule)
			} else {
				httpRules = append(httpRules, *eRouteCtx.Spec.Rules[ruleIdx].DeepCopy())
			}
		}

		if !hasRedirect {
			eir.HTTPRoutes[key] = eRouteCtx
			continue
		}

		// For the HTTP route, combine trailing slash + SSL upgrade into single-hop
		// redirects: http://host/path -> 301 https://host/path/
		// Ingress nginx does http://host/path -> http://host/path/ -> https://host/path -> https://host/path/.
		for _, tsRule := range trailingSlashRules {
			combinedRule := *tsRule.DeepCopy()
			combinedRule.Filters[0].RequestRedirect.Scheme = ptr.To("https")
			httpRules = append(httpRules, combinedRule)
		}

		redirectRoute := gatewayv1.HTTPRoute{
			TypeMeta: httpRouteContext.HTTPRoute.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-http", httpRouteContext.HTTPRoute.Name),
				Namespace: httpRouteContext.HTTPRoute.Namespace,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				Hostnames: httpRouteContext.HTTPRoute.Spec.DeepCopy().Hostnames,
				Rules:     httpRules,
			},
		}
		// Add parentrefs.
		redirectRoute.Spec.ParentRefs = httpRouteContext.HTTPRoute.Spec.DeepCopy().ParentRefs
		// Bind to port 80.
		for i := range redirectRoute.Spec.ParentRefs {
			redirectRoute.Spec.ParentRefs[i].Port = ptr.To[int32](80)
		}
		eir.HTTPRoutes[types.NamespacedName{
			Namespace: redirectRoute.Namespace,
			Name:      redirectRoute.Name,
		}] = emitterir.HTTPRouteContext{
			HTTPRoute: redirectRoute,
		}

		// Bind original route to port 443.
		for i := range eRouteCtx.Spec.ParentRefs {
			eRouteCtx.Spec.ParentRefs[i].Port = ptr.To[int32](443)
		}
		eir.HTTPRoutes[key] = eRouteCtx
	}
}

// isValidTemporalRedirectCode returns true if the code is in the intersection of
// ingress-nginx temporal-redirect codes (300-307) and Gateway API codes (301,302,303,307,308).
// Result: 301, 302, 303, 307.
func isValidTemporalRedirectCode(code int) bool {
	switch code {
	case 301, 302, 303, 307:
		return true
	default:
		return false
	}
}

// isValidPermanentRedirectCode returns true if the code is in the intersection of
// ingress-nginx permanent-redirect codes (300-308) and Gateway API codes (301,302,303,307,308).
// Result: 301, 302, 303, 307, 308.
func isValidPermanentRedirectCode(code int) bool {
	switch code {
	case 301, 302, 303, 307, 308:
		return true
	default:
		return false
	}
}

// buildTrailingSlashRedirectRules generates HTTPRouteRules that redirect /path
// to /path/ with a 301, matching NGINX's implicit trailing slash behavior.
// Rules are only generated when a trailing-slash path exists in the input and
// no existing rule already covers the non-slash variant.
func buildTrailingSlashRedirectRules(rules []gatewayv1.HTTPRouteRule) []gatewayv1.HTTPRouteRule {
	sourcePathAlreadyMatched := make(map[string]struct{})
	var existingPrefixes []string
	for _, rule := range rules {
		for _, match := range rule.Matches {
			if match.Path == nil || match.Path.Type == nil || match.Path.Value == nil {
				continue
			}
			if *match.Path.Type == gatewayv1.PathMatchExact || *match.Path.Type == gatewayv1.PathMatchPathPrefix {
				sourcePathAlreadyMatched[*match.Path.Value] = struct{}{}
			}
			if *match.Path.Type == gatewayv1.PathMatchPathPrefix {
				existingPrefixes = append(existingPrefixes, *match.Path.Value)
			}
		}
	}

	var redirectRules []gatewayv1.HTTPRouteRule
	redirectAdded := make(map[string]struct{})
	for _, rule := range rules {
		for _, match := range rule.Matches {
			if match.Path == nil || match.Path.Type == nil || match.Path.Value == nil {
				continue
			}

			matchType := *match.Path.Type
			if matchType != gatewayv1.PathMatchExact && matchType != gatewayv1.PathMatchPathPrefix {
				continue
			}

			redirectTarget := *match.Path.Value
			if redirectTarget == "/" || !strings.HasSuffix(redirectTarget, "/") {
				continue
			}

			redirectSource := strings.TrimSuffix(redirectTarget, "/")
			if redirectSource == "" {
				continue
			}
			if _, exists := sourcePathAlreadyMatched[redirectSource]; exists {
				continue
			}
			if pathPrefixCoveredByAny(existingPrefixes, redirectSource) {
				continue
			}
			if _, added := redirectAdded[redirectSource]; added {
				continue
			}

			exact := gatewayv1.PathMatchExact
			redirectRules = append(redirectRules, gatewayv1.HTTPRouteRule{
				Matches: []gatewayv1.HTTPRouteMatch{
					{
						Path: &gatewayv1.HTTPPathMatch{
							Type:  &exact,
							Value: ptr.To(redirectSource),
						},
					},
				},
				Filters: []gatewayv1.HTTPRouteFilter{
					{
						Type: gatewayv1.HTTPRouteFilterRequestRedirect,
						RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
							StatusCode: ptr.To(301),
							Path: &gatewayv1.HTTPPathModifier{
								Type:            gatewayv1.FullPathHTTPPathModifier,
								ReplaceFullPath: ptr.To(redirectTarget),
							},
						},
					},
				},
			})

			redirectAdded[redirectSource] = struct{}{}
		}
	}

	return redirectRules
}

// pathPrefixCoveredByAny returns true if any existing PathPrefix would match
// the given path using Gateway API segment-based prefix matching semantics.
func pathPrefixCoveredByAny(prefixes []string, path string) bool {
	for _, prefix := range prefixes {
		if pathPrefixCovers(prefix, path) {
			return true
		}
	}
	return false
}

// pathPrefixCovers returns true if a Gateway API PathPrefix match for prefix
// would match path. Gateway API PathPrefix matching is segment-based:
// PathPrefix "/a" matches "/a", "/a/", "/a/b" but NOT "/ab".
func pathPrefixCovers(prefix, path string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	if len(path) == len(prefix) {
		return true
	}
	// Path is longer than prefix; check segment boundary.
	return prefix[len(prefix)-1] == '/' || path[len(prefix)] == '/'
}
