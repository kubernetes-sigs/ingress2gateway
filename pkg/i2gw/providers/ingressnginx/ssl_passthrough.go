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
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// sslPassthroughFeature converts the nginx.ingress.kubernetes.io/ssl-passthrough
// annotation into a Gateway API TLSRoute with a TLS Passthrough listener.
//
// When ssl-passthrough is enabled on an Ingress, TLS connections are sent
// directly to the backend without decryption. In Gateway API this maps to a
// Gateway listener with protocol TLS and mode Passthrough, paired with a
// TLSRoute that forwards traffic based on SNI hostname.
//
// Because SSL passthrough operates at L4, it invalidates all other L7
// annotations. This feature parser therefore removes the corresponding
// HTTPRoute entries from the IR and replaces them with TLSRoutes.
func sslPassthroughFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	// Build a set of ingresses that have ssl-passthrough enabled.
	passthroughIngresses := map[types.NamespacedName]*networkingv1.Ingress{}
	for i := range ingresses {
		ing := &ingresses[i]
		val, ok := ing.Annotations[SSLPassthroughAnnotation]
		if !ok {
			continue
		}
		enabled, err := strconv.ParseBool(val)
		if err != nil || !enabled {
			continue
		}
		key := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
		passthroughIngresses[key] = ing
	}

	if len(passthroughIngresses) == 0 {
		return nil
	}

	// Identify HTTPRoutes whose rules all come from passthrough ingresses.
	// For each such route, create a TLSRoute and remove the HTTPRoute.
	httpRoutesToDelete := []types.NamespacedName{}

	for routeKey, httpRouteCtx := range ir.HTTPRoutes {
		// Check if ALL backend sources for this route come from passthrough ingresses.
		allPassthrough := true
		for _, ruleSources := range httpRouteCtx.RuleBackendSources {
			for _, src := range ruleSources {
				if src.Ingress == nil {
					allPassthrough = false
					break
				}
				ingKey := types.NamespacedName{Namespace: src.Ingress.Namespace, Name: src.Ingress.Name}
				if _, ok := passthroughIngresses[ingKey]; !ok {
					allPassthrough = false
					break
				}
			}
			if !allPassthrough {
				break
			}
		}

		if !allPassthrough {
			// Some rules come from non-passthrough ingresses. Warn and skip.
			// This handles edge cases where hosts are shared across ingresses.
			for _, ruleSources := range httpRouteCtx.RuleBackendSources {
				for _, src := range ruleSources {
					if src.Ingress == nil {
						continue
					}
					ingKey := types.NamespacedName{Namespace: src.Ingress.Namespace, Name: src.Ingress.Name}
					if _, ok := passthroughIngresses[ingKey]; ok {
						notify(notifications.WarningNotification, fmt.Sprintf(
							"Ingress %s/%s has ssl-passthrough enabled but shares a host with non-passthrough Ingress resources. "+
								"The ssl-passthrough annotation will be ignored for this host.",
							src.Ingress.Namespace, src.Ingress.Name), src.Ingress)
					}
				}
			}
			continue
		}

		// All sources are passthrough — create a TLSRoute.
		tlsRoute := buildTLSRoute(httpRouteCtx)
		tlsRouteKey := types.NamespacedName{Namespace: tlsRoute.Namespace, Name: tlsRoute.Name}
		ir.TLSRoutes[tlsRouteKey] = tlsRoute

		// Add TLS Passthrough listener to the Gateway.
		addPassthroughListeners(ir, httpRouteCtx)

		httpRoutesToDelete = append(httpRoutesToDelete, routeKey)

		// Notify about L7 annotations being invalidated.
		for _, ruleSources := range httpRouteCtx.RuleBackendSources {
			for _, src := range ruleSources {
				if src.Ingress == nil {
					continue
				}
				notify(notifications.InfoNotification, fmt.Sprintf(
					"Ingress %s/%s uses ssl-passthrough: converting to TLSRoute (L7 annotations are ignored for passthrough backends).",
					src.Ingress.Namespace, src.Ingress.Name), src.Ingress)
			}
		}
	}

	// Remove the HTTPRoutes that have been replaced by TLSRoutes.
	for _, key := range httpRoutesToDelete {
		delete(ir.HTTPRoutes, key)
	}

	return nil
}

// buildTLSRoute creates a TLSRoute from an HTTPRoute context. It uses the
// hostnames and backend references from the HTTPRoute, keeping the same
// parent refs so it attaches to the same Gateway.
func buildTLSRoute(httpRouteCtx providerir.HTTPRouteContext) gatewayv1alpha2.TLSRoute {
	tlsRoute := gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteCtx.HTTPRoute.Name + "-tls-passthrough",
			Namespace: httpRouteCtx.HTTPRoute.Namespace,
		},
		Spec: gatewayv1alpha2.TLSRouteSpec{
			Hostnames: make([]gatewayv1.Hostname, len(httpRouteCtx.HTTPRoute.Spec.Hostnames)),
		},
		Status: gatewayv1alpha2.TLSRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{},
			},
		},
	}
	tlsRoute.SetGroupVersionKind(common.TLSRouteGVK)

	copy(tlsRoute.Spec.Hostnames, httpRouteCtx.HTTPRoute.Spec.Hostnames)

	// Set parent refs pointing to the TLS passthrough listener section.
	for _, parentRef := range httpRouteCtx.HTTPRoute.Spec.ParentRefs {
		tlsParentRef := gatewayv1.ParentReference{
			Name:      parentRef.Name,
			Namespace: parentRef.Namespace,
			Group:     parentRef.Group,
		}
		// Point to the passthrough listener section.
		if len(tlsRoute.Spec.Hostnames) > 0 {
			sectionName := gatewayv1.SectionName(fmt.Sprintf("%s-tls-passthrough",
				common.NameFromHost(string(tlsRoute.Spec.Hostnames[0]))))
			tlsParentRef.SectionName = &sectionName
		}
		tlsRoute.Spec.ParentRefs = append(tlsRoute.Spec.ParentRefs, tlsParentRef)
	}

	// Convert HTTP backend refs to TLS backend refs.
	// SSL passthrough sends all traffic to the backend, so we consolidate
	// all unique backends into a single rule.
	seenBackends := map[string]struct{}{}
	var backendRefs []gatewayv1.BackendRef

	for _, rule := range httpRouteCtx.HTTPRoute.Spec.Rules {
		for _, httpBackendRef := range rule.BackendRefs {
			key := fmt.Sprintf("%s/%d", httpBackendRef.Name, *httpBackendRef.BackendRef.Port)
			if _, exists := seenBackends[key]; exists {
				continue
			}
			seenBackends[key] = struct{}{}
			backendRefs = append(backendRefs, httpBackendRef.BackendRef)
		}
	}

	if len(backendRefs) > 0 {
		tlsRoute.Spec.Rules = []gatewayv1alpha2.TLSRouteRule{
			{BackendRefs: backendRefs},
		}
	}

	return tlsRoute
}

// addPassthroughListeners adds TLS Passthrough listeners to the Gateway(s)
// referenced by the HTTPRoute. These listeners use the TLS protocol with
// Passthrough mode, matching the SNI hostname.
func addPassthroughListeners(ir *providerir.ProviderIR, httpRouteCtx providerir.HTTPRouteContext) {
	for _, parentRef := range httpRouteCtx.HTTPRoute.Spec.ParentRefs {
		gwNamespace := httpRouteCtx.HTTPRoute.Namespace
		if parentRef.Namespace != nil {
			gwNamespace = string(*parentRef.Namespace)
		}
		gwKey := types.NamespacedName{
			Namespace: gwNamespace,
			Name:      string(parentRef.Name),
		}

		gwCtx, ok := ir.Gateways[gwKey]
		if !ok {
			continue
		}

		for _, hostname := range httpRouteCtx.HTTPRoute.Spec.Hostnames {
			listenerName := gatewayv1.SectionName(fmt.Sprintf("%s-tls-passthrough",
				common.NameFromHost(string(hostname))))

			// Check if this listener already exists.
			exists := false
			for _, l := range gwCtx.Gateway.Spec.Listeners {
				if l.Name == listenerName {
					exists = true
					break
				}
			}
			if exists {
				continue
			}

			h := hostname
			gwCtx.Gateway.Spec.Listeners = append(gwCtx.Gateway.Spec.Listeners, gatewayv1.Listener{
				Name:     listenerName,
				Hostname: &h,
				Port:     443,
				Protocol: gatewayv1.TLSProtocolType,
				TLS: &gatewayv1.ListenerTLSConfig{
					Mode: common.PtrTo(gatewayv1.TLSModePassthrough),
				},
			})
		}

		ir.Gateways[gwKey] = gwCtx
	}
}
