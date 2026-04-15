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

package traefik

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Traefik entrypoint names that map to standard HTTP/HTTPS ports.
const (
	entrypointWeb       = "web"       // port 80, plain HTTP
	entrypointWebsecure = "websecure" // port 443, HTTPS
)

// routerEntrypointsFeature translates the
// traefik.ingress.kubernetes.io/router.entrypoints annotation.
//
// Traefik entrypoints define which ports a router listens on. The two standard
// entrypoints are:
//   - web       -> port 80  (HTTP)
//   - websecure -> port 443 (HTTPS)
//
// When only "websecure" is listed (without "web"), the route is HTTPS-only.
// If an HTTPS listener is already present on the Gateway, the HTTP listener is
// kept so that forceHTTPSFeature can attach a redirect HTTPRoute to it.
// If no HTTPS listener exists, the HTTP listener is removed as a fallback.
//
// When only "web" is listed, the route is HTTP-only -- the HTTPS listener (if
// any was added by router_tls.go) is removed.
//
// When both are listed, or when neither standard entrypoint is present, no
// listener is removed and an info notification is emitted for non-standard
// entrypoints so the user can review the Gateway configuration manually.
func routerEntrypointsFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			val, ok := rule.Ingress.Annotations[RouterEntrypointsAnnotation]
			if !ok {
				continue
			}

			entrypoints := parseEntrypoints(val)
			hasWeb := entrypoints[entrypointWeb]
			hasWebsecure := entrypoints[entrypointWebsecure]

			// common.ToIR() names the Gateway after the ingressClass, not the ingress name.
			gatewayKey := types.NamespacedName{
				Namespace: rule.Ingress.Namespace,
				Name:      rg.IngressClass,
			}
			gw, found := ir.Gateways[gatewayKey]
			if !found {
				errs = append(errs, field.NotFound(
					field.NewPath("Gateway"),
					fmt.Sprintf("%s (from ingress %s/%s)", gatewayKey, rule.Ingress.Namespace, rule.Ingress.Name),
				))
				continue
			}

			routeKey := types.NamespacedName{
				Namespace: rule.Ingress.Namespace,
				Name:      common.RouteName(rg.Name, rg.Host),
			}
			httpRoute := ir.HTTPRoutes[routeKey]

			switch {
			case hasWebsecure && !hasWeb:
				// HTTPS-only route. When an HTTPS listener is already present on the
				// Gateway (added by router_tls.go or via spec.tls), keep the HTTP
				// listener so that forceHTTPSFeature can attach a redirect HTTPRoute
				// to it. If no HTTPS listener exists there is nothing to redirect to,
				// so the HTTP listener is removed as a fallback.
				httpSectionName := gatewayv1.SectionName(httpListenerName(rg.Host))

				hasHTTPS := false
				for _, l := range gw.Spec.Listeners {
					if l.Protocol == gatewayv1.HTTPSProtocolType {
						hasHTTPS = true
						break
					}
				}

				if hasHTTPS {
					notify(
						notifications.InfoNotification,
						fmt.Sprintf(
							"parsed %q annotation (value: %q): route is HTTPS-only -- "+
								"HTTP listener %q kept for HTTP->HTTPS redirect",
							RouterEntrypointsAnnotation, val, httpSectionName,
						),
						&httpRoute.HTTPRoute,
					)
				} else {
					gw.Spec.Listeners = removeListener(gw.Spec.Listeners, httpSectionName)
					ir.Gateways[gatewayKey] = gw
					notify(
						notifications.InfoNotification,
						fmt.Sprintf(
							"parsed %q annotation (value: %q): removed HTTP listener %q from Gateway %s -- route is HTTPS-only",
							RouterEntrypointsAnnotation, val, httpSectionName, gatewayKey,
						),
						&httpRoute.HTTPRoute,
					)
				}

			case hasWeb && !hasWebsecure:
				// HTTP-only: remove the HTTPS listener if one was added (e.g. by router_tls.go).
				httpsName := gatewayv1.SectionName(httpsListenerName(rg.Host))
				gw.Spec.Listeners = removeListener(gw.Spec.Listeners, httpsName)
				ir.Gateways[gatewayKey] = gw
				notify(
					notifications.InfoNotification,
					fmt.Sprintf(
						"parsed %q annotation (value: %q): removed HTTPS listener %q from Gateway %s -- route is HTTP-only",
						RouterEntrypointsAnnotation, val, httpsName, gatewayKey,
					),
					&httpRoute.HTTPRoute,
				)

			case hasWeb && hasWebsecure:
				// Both entrypoints -- nothing to remove, this is the default Gateway API
				// behavior (HTTP + HTTPS listeners). No notification needed.

			default:
				// Non-standard entrypoint name (e.g. a custom Traefik entrypoint like
				// "internal" or "metrics"). We cannot map this automatically.
				notify(
					notifications.WarningNotification,
					fmt.Sprintf(
						"parsed %q annotation (value: %q) on ingress %s/%s: "+
							"non-standard entrypoint(s) cannot be mapped to Gateway listeners -- "+
							"review the Gateway spec and add the correct listener port/protocol manually",
						RouterEntrypointsAnnotation, val,
						rule.Ingress.Namespace, rule.Ingress.Name,
					),
				)
			}
		}
	}
	return errs
}

// parseEntrypoints splits a comma-separated entrypoints annotation value into a
// set of lowercase trimmed names.
func parseEntrypoints(val string) map[string]bool {
	result := make(map[string]bool)
	for _, ep := range strings.Split(val, ",") {
		name := strings.ToLower(strings.TrimSpace(ep))
		if name != "" {
			result[name] = true
		}
	}
	return result
}

// httpListenerName returns the name that common.ToIR() assigns to the HTTP
// listener for a given host. The format mirrors converter.go:
//
//	fmt.Sprintf("%shttp", listenerNamePrefix)
//
// where listenerNamePrefix is either "" (no host) or "{NameFromHost(host)}-".
func httpListenerName(host string) string {
	if host == "" {
		return "http"
	}
	return fmt.Sprintf("%s-http", common.NameFromHost(host))
}

// httpsListenerName returns the name that common.ToIR() / router_tls.go assigns
// to the HTTPS listener for a given host.
func httpsListenerName(host string) string {
	if host == "" {
		return "https"
	}
	return fmt.Sprintf("%s-https", common.NameFromHost(host))
}

// removeListener returns a new slice with the listener named name removed.
// If no listener with that name exists the original slice is returned unchanged.
func removeListener(listeners []gatewayv1.Listener, name gatewayv1.SectionName) []gatewayv1.Listener {
	result := make([]gatewayv1.Listener, 0, len(listeners))
	for _, l := range listeners {
		if l.Name != name {
			result = append(result, l)
		}
	}
	return result
}
