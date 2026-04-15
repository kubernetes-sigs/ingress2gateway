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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// forceHTTPSFeature generates an HTTP->HTTPS redirect HTTPRoute for Traefik
// ingresses that are configured as HTTPS-only.
//
// It handles two cases:
//
//  1. entrypoints=websecure: routerEntrypointsFeature already kept the HTTP
//     listener in this case (when an HTTPS listener is present). This feature
//     attaches a redirect HTTPRoute to that HTTP listener, so all port-80
//     traffic receives a 301 redirect to the HTTPS equivalent URL.
//
//  2. Future: a dedicated traefik.ingress.kubernetes.io/force-https annotation
//     could be added here for ingresses that don't use the entrypoints
//     annotation but still need HTTP->HTTPS redirect.
//
// The generated redirect HTTPRoute is named "{original-route-name}-http" and is
// bound to the HTTP listener via sectionName. This matches the pattern used by
// the ingress-nginx provider.
func forceHTTPSFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			val, ok := rule.Ingress.Annotations[RouterEntrypointsAnnotation]
			if !ok {
				continue
			}

			entrypoints := parseEntrypoints(val)
			if !entrypoints[entrypointWebsecure] || entrypoints[entrypointWeb] {
				// Only act when websecure is set without web (HTTPS-only intent).
				continue
			}

			gatewayKey := types.NamespacedName{
				Namespace: rule.Ingress.Namespace,
				Name:      rg.IngressClass,
			}
			gw, found := ir.Gateways[gatewayKey]
			if !found {
				// Gateway not found -- routerEntrypointsFeature already reported this.
				continue
			}

			// Check that the HTTP listener was kept (i.e. an HTTPS listener exists).
			// If routerEntrypointsFeature removed the HTTP listener (fallback), there
			// is nothing to attach a redirect route to.
			httpSectionName := gatewayv1.SectionName(httpListenerName(rg.Host))
			hasHTTPListener := false
			for _, l := range gw.Spec.Listeners {
				if l.Name == httpSectionName && l.Protocol == gatewayv1.HTTPProtocolType {
					hasHTTPListener = true
					break
				}
			}
			if !hasHTTPListener {
				continue
			}

			routeKey := types.NamespacedName{
				Namespace: rule.Ingress.Namespace,
				Name:      common.RouteName(rg.Name, rg.Host),
			}
			redirectRouteKey := types.NamespacedName{
				Namespace: routeKey.Namespace,
				Name:      routeKey.Name + "-http",
			}

			ir.HTTPRoutes[redirectRouteKey] = providerir.HTTPRouteContext{
				HTTPRoute: buildHTTPSRedirectRoute(
					redirectRouteKey.Name,
					redirectRouteKey.Namespace,
					rg.Host,
					gatewayKey,
					httpSectionName,
				),
			}

			// Pin the main HTTPRoute to the HTTPS listener so it does not also
			// match HTTP traffic and conflict with the redirect route.
			httpsSectionName := gatewayv1.SectionName(httpsListenerName(rg.Host))
			gwNamespace := gatewayv1.Namespace(gatewayKey.Namespace)
			httpRoute := ir.HTTPRoutes[routeKey]
			for i, ref := range httpRoute.Spec.ParentRefs {
				if string(ref.Name) == gatewayKey.Name && ref.SectionName == nil {
					httpRoute.Spec.ParentRefs[i].SectionName = &httpsSectionName
					httpRoute.Spec.ParentRefs[i].Namespace = &gwNamespace
				}
			}
			ir.HTTPRoutes[routeKey] = httpRoute

			notify(
				notifications.InfoNotification,
				fmt.Sprintf(
					"added HTTP->HTTPS redirect HTTPRoute %q bound to listener %q; pinned main HTTPRoute to HTTPS listener %q",
					redirectRouteKey, httpSectionName, httpsSectionName,
				),
				&httpRoute.HTTPRoute,
			)
		}
	}
	return errs
}

// buildHTTPSRedirectRoute returns an HTTPRoute that redirects all HTTP traffic
// to HTTPS (301). It is bound to the named HTTP listener on the given Gateway
// via sectionName so that only port-80 traffic is affected.
func buildHTTPSRedirectRoute(name, namespace, host string, gatewayKey types.NamespacedName, sectionName gatewayv1.SectionName) gatewayv1.HTTPRoute {
	var hostnames []gatewayv1.Hostname
	if host != "" {
		hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(host)}
	}
	gwNamespace := gatewayv1.Namespace(gatewayKey.Namespace)
	route := gatewayv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.networking.k8s.io/v1",
			Kind:       "HTTPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Hostnames: hostnames,
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
								Scheme:     ptr.To("https"),
								StatusCode: ptr.To(301),
							},
						},
					},
				},
			},
		},
	}
	route.Spec.ParentRefs = []gatewayv1.ParentReference{
		{
			Name:        gatewayv1.ObjectName(gatewayKey.Name),
			Namespace:   &gwNamespace,
			SectionName: &sectionName,
		},
	}
	return route
}
