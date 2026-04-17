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
)

// sslPassthroughFeature converts the nginx.ingress.kubernetes.io/ssl-passthrough
// annotation into a Gateway API TLSRoute with a TLS Passthrough listener.
//
// When ssl-passthrough is enabled on an Ingress, TLS connections are sent
// directly to the backend without decryption. In Gateway API this maps to a
// Gateway listener with protocol TLS and mode Passthrough, paired with a
// TLSRoute that forwards traffic based on SNI hostname.
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

	httpRoutesToDelete := []types.NamespacedName{}

	for routeKey, httpRouteCtx := range ir.HTTPRoutes {
		hasPassthrough := false
		hasNonPassthrough := false
		for _, ruleSources := range httpRouteCtx.RuleBackendSources {
			for _, src := range ruleSources {
				if src.Ingress == nil {
					hasNonPassthrough = true
					continue
				}
				ingKey := types.NamespacedName{Namespace: src.Ingress.Namespace, Name: src.Ingress.Name}
				if _, ok := passthroughIngresses[ingKey]; ok {
					hasPassthrough = true
				} else {
					hasNonPassthrough = true
				}
			}
		}

		if !hasPassthrough {
			continue
		}

		// Create a TLSRoute with only backends from passthrough ingresses.
		tlsRoute := buildTLSRoute(httpRouteCtx, passthroughIngresses)
		tlsRouteKey := types.NamespacedName{Namespace: tlsRoute.Namespace, Name: tlsRoute.Name}
		ir.TLSRoutes[tlsRouteKey] = tlsRoute

		// Add TLS Passthrough listener to the Gateway.
		addPassthroughListeners(ir, httpRouteCtx)

		if hasNonPassthrough {
			// Strip rules sourced entirely from passthrough ingresses,
			// keeping rules from non-passthrough ingresses for HTTP.
			stripPassthroughRules(&httpRouteCtx, passthroughIngresses)
			ir.HTTPRoutes[routeKey] = httpRouteCtx
		} else {
			// All rules come from passthrough ingresses — remove the HTTPRoute.
			httpRoutesToDelete = append(httpRoutesToDelete, routeKey)
		}
	}

	// Remove HTTPRoutes that have been fully replaced by TLSRoutes.
	for _, key := range httpRoutesToDelete {
		delete(ir.HTTPRoutes, key)
	}

	return nil
}

// stripPassthroughRules removes backends sourced from passthrough ingresses
// from the HTTPRoute rules. Rules that end up with zero backends are removed
// entirely. This preserves the HTTP (port 80) routing for non-passthrough
// ingresses that share the same hostname.
func stripPassthroughRules(ctx *providerir.HTTPRouteContext, passthroughIngresses map[types.NamespacedName]*networkingv1.Ingress) {
	var newRules []gatewayv1.HTTPRouteRule
	var newSources [][]providerir.BackendSource

	for ruleIdx, rule := range ctx.HTTPRoute.Spec.Rules {
		var keptBackends []gatewayv1.HTTPBackendRef
		var keptSources []providerir.BackendSource

		for backendIdx, backendRef := range rule.BackendRefs {
			isPassthrough := false
			if ruleIdx < len(ctx.RuleBackendSources) && backendIdx < len(ctx.RuleBackendSources[ruleIdx]) {
				src := ctx.RuleBackendSources[ruleIdx][backendIdx]
				if src.Ingress != nil {
					ingKey := types.NamespacedName{Namespace: src.Ingress.Namespace, Name: src.Ingress.Name}
					if _, ok := passthroughIngresses[ingKey]; ok {
						isPassthrough = true
					}
				}
			}
			if !isPassthrough {
				keptBackends = append(keptBackends, backendRef)
				if ruleIdx < len(ctx.RuleBackendSources) && backendIdx < len(ctx.RuleBackendSources[ruleIdx]) {
					keptSources = append(keptSources, ctx.RuleBackendSources[ruleIdx][backendIdx])
				}
			}
		}

		if len(keptBackends) > 0 {
			rule.BackendRefs = keptBackends
			newRules = append(newRules, rule)
			newSources = append(newSources, keptSources)
		}
	}

	ctx.HTTPRoute.Spec.Rules = newRules
	ctx.RuleBackendSources = newSources
}

// buildTLSRoute creates a TLSRoute from an HTTPRoute context. It uses the
// hostnames and backend references from the HTTPRoute, keeping the same
// parent refs so it attaches to the same Gateway. Only backends that originate
// from passthrough ingresses are included in the TLSRoute.
func buildTLSRoute(httpRouteCtx providerir.HTTPRouteContext, passthroughIngresses map[types.NamespacedName]*networkingv1.Ingress) gatewayv1.TLSRoute {
	tlsRoute := gatewayv1.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteCtx.HTTPRoute.Name + "-tls-passthrough",
			Namespace: httpRouteCtx.HTTPRoute.Namespace,
		},
		Spec: gatewayv1.TLSRouteSpec{
			Hostnames: make([]gatewayv1.Hostname, len(httpRouteCtx.HTTPRoute.Spec.Hostnames)),
		},
		Status: gatewayv1.TLSRouteStatus{
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
	// all unique backends into a single rule. Only backends sourced from
	// passthrough ingresses are included.
	seenBackends := map[string]struct{}{}
	var backendRefs []gatewayv1.BackendRef

	for ruleIdx, rule := range httpRouteCtx.HTTPRoute.Spec.Rules {
		for backendIdx, httpBackendRef := range rule.BackendRefs {
			// Filter: only include backends from passthrough ingresses.
			if ruleIdx < len(httpRouteCtx.RuleBackendSources) &&
				backendIdx < len(httpRouteCtx.RuleBackendSources[ruleIdx]) {
				src := httpRouteCtx.RuleBackendSources[ruleIdx][backendIdx]
				if src.Ingress != nil {
					ingKey := types.NamespacedName{Namespace: src.Ingress.Namespace, Name: src.Ingress.Name}
					if _, ok := passthroughIngresses[ingKey]; !ok {
						continue
					}
				}
			}
			key := fmt.Sprintf("%s/%d", httpBackendRef.Name, *httpBackendRef.BackendRef.Port)
			if _, exists := seenBackends[key]; exists {
				continue
			}
			seenBackends[key] = struct{}{}
			backendRefs = append(backendRefs, httpBackendRef.BackendRef)
		}
	}

	if len(backendRefs) > 0 {
		tlsRoute.Spec.Rules = []gatewayv1.TLSRouteRule{
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
