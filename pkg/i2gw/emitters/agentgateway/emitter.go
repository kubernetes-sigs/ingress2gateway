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

package agentgateway

import (
	"fmt"
	"sort"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitters/utils"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	"github.com/agentgateway/agentgateway/controller/api/v1alpha1/shared"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const gatewayClassName = "agentgateway"

func init() {
	i2gw.EmitterConstructorByName["agentgateway"] = NewEmitter
}

type Emitter struct{}

// NewEmitter returns a new instance of AgentgatewayEmitter.
func NewEmitter(_ *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{}
}

// Emit converts EmitterIR to Gateway API resources plus agentgateway-specific extensions.
func (e *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) > 0 {
		return gatewayResources, errs
	}

	// Set GatewayClassName to "agentgateway" for all Gateways
	for key := range gatewayResources.Gateways {
		gateway := gatewayResources.Gateways[key]
		gateway.Spec.GatewayClassName = gatewayClassName
		gatewayResources.Gateways[key] = gateway
	}

	// Track agentgateway-specific resources
	var agentgatewayObjs []client.Object

	// Track AgentgatewayPolicies per ingress name
	agentgatewayPolicies := map[string]*agentgatewayv1alpha1.AgentgatewayPolicy{}

	// Track backend-scoped AgentgatewayPolicies per Service (ns/name) (e.g. TLS, connect timeout)
	backendPolicies := map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy{}

	// Track AgentgatewayBackends per Service-upstream backend key (ns/name).
	agentgatewayBackends := map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayBackend{}

	// Track HTTPRoutes that need SSL redirect splitting
	routesToSplitForSSLRedirect := map[types.NamespacedName]bool{}

	// De-dupe INFO notifications across routes/policies.
	basicAuthSecretSeen := map[basicAuthSecretKey]struct{}{}

	for httpRouteKey, httpRouteContext := range ir.HTTPRoutes {
		if len(httpRouteContext.PoliciesBySourceIngressName) == 0 {
			continue
		}

		// Apply host-wide regex enforcement first (so rule path regex is finalized)
		// TODO: implement regex path matching if needed

		// deterministic policy iteration
		policyNames := make([]string, 0, len(httpRouteContext.PoliciesBySourceIngressName))
		for name := range httpRouteContext.PoliciesBySourceIngressName {
			policyNames = append(policyNames, name)
		}
		sort.Strings(policyNames)

		for _, polSourceIngressName := range policyNames {
			pol := httpRouteContext.PoliciesBySourceIngressName[polSourceIngressName]

			// Normalize (rule, backend) coverage to unique pairs to avoid
			// generating duplicate filters on the same backendRef.
			coverage := uniquePolicyIndices(pol.RuleBackendSources)

			touched := false
			corsTouched := false

			// Apply rate limit policy features that map to AgentgatewayPolicy.
			if applyRateLimitPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, agentgatewayPolicies) {
				touched = true
			}

			// Apply timeout policy features that map to AgentgatewayPolicy.
			if applyRequestTimeoutPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, agentgatewayPolicies) {
				touched = true
			}

			// Check if SSL redirect is enabled but don't apply it yet (will split route later).
			if applySSLRedirectPolicy(pol) {
				routesToSplitForSSLRedirect[httpRouteKey] = true
			}

			// Proxy connect timeout maps to AgentgatewayPolicy.spec.backend.tcp.connectTimeout, targeting Services.
			applyProxyConnectTimeoutPolicy(
				pol,
				polSourceIngressName,
				httpRouteKey,
				httpRouteContext,
				agentgatewayPolicies,
				backendPolicies,
			)

			// rewrite-target maps to AgentgatewayPolicy.spec.traffic.transformation.
			// Note: agentgateway attaches policies at the HTTPRoute scope; this feature is only safe when
			// it fully covers the route (enforced by the full-coverage check below).
			if applyRewriteTargetPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, &httpRouteContext, agentgatewayPolicies) {
				touched = true
			}

			// CORS maps to AgentgatewayPolicy.spec.traffic.cors.
			if applyCorsPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, agentgatewayPolicies) {
				touched, corsTouched = true, true
			}

			// enable-access-log maps to AgentgatewayPolicy.spec.frontend.accessLog.
			if applyAccessLogPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, agentgatewayPolicies) {
				touched = true
			}

			// ExtAuth maps to AgentgatewayPolicy.spec.traffic.extAuth.
			if applyExtAuthPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, agentgatewayPolicies) {
				touched = true
			}

			// Backend TLS maps to AgentgatewayPolicy.spec.backend.tls, targeting the covered Service backends.
			// Note: this emits per-Service policies (like kgateway's BackendConfigPolicy approach).
			applyBackendTLSPolicy(
				pol,
				httpRouteKey,
				httpRouteContext,
				backendPolicies,
			)

			// service-upstream maps to AgentgatewayBackend (spec.static) and rewrites HTTPRoute backendRefs.
			// Keep this after Service-targeted backend policy projection so those policies still target Services.
			applyServiceUpstream(
				pol,
				polSourceIngressName,
				httpRouteKey,
				&httpRouteContext,
				agentgatewayBackends,
			)

			// backend-protocol maps to AgentgatewayPolicy.spec.backend.http.version.
			// If service-upstream rewrote backendRefs to AgentgatewayBackend, target those backends.
			// Otherwise, target core Services.
			applyBackendProtocolPolicy(
				pol,
				httpRouteKey,
				httpRouteContext,
				backendPolicies,
			)

			// BasicAuth maps to AgentgatewayPolicy.spec.traffic.basicAuthentication.
			// Note: agentgateway expects htpasswd content under a '.htaccess' key; see BasicAuthentication docs.
			if applyBasicAuthPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, agentgatewayPolicies) {
				touched = true
				// Emit an INFO notification with guidance about Secret key expectations.
				emitBasicAuthSecretNotifications(
					pol,
					polSourceIngressName,
					httpRouteKey.Namespace,
					basicAuthSecretSeen,
				)
			}

			// Buffer policy maps to AgentgatewayPolicy.spec.frontend.http.maxBufferSize.
			if bufferTouched, bufferErr := applyBufferPolicy(
				pol,
				polSourceIngressName,
				httpRouteKey.Namespace,
				agentgatewayPolicies,
			); bufferErr != nil {
				errs = append(errs, bufferErr)
			} else if bufferTouched {
				touched = true
			}

			// Attach the resulting AgentgatewayPolicy to the HTTPRoute, but only if the policy fully covers the route.
			// Unlike kgateway, agentgateway does not support attaching policies via HTTPRoute filter ExtensionRef.
			if touched {
				agp := agentgatewayPolicies[polSourceIngressName]
				if agp != nil {
					total := numRules(httpRouteContext.HTTPRoute)
					covered := len(coverage)
					// Some ingress-nginx features are recorded at the Ingress scope (not per rule/backend pair).
					// In that case RuleBackendSources may be empty; treat this as "applies to all backends".
					// This avoids false "subset coverage" errors like (0/1) for Ingress-wide annotations.
					if covered == 0 {
						covered = total
					}

					if covered == total {
						agp.Spec.TargetRefs = []shared.LocalPolicyTargetReferenceWithSectionName{{
							LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
								Group: gatewayv1.Group("gateway.networking.k8s.io"),
								Kind:  gatewayv1.Kind("HTTPRoute"),
								Name:  gatewayv1.ObjectName(httpRouteKey.Name),
							},
						}}
						// Strip upstream CORS headers when CORS is enabled and policy is attached.
						if corsTouched {
							utils.EnsureStripUpstreamCORSHeaders(&httpRouteContext.HTTPRoute)
						}
					} else {
						errs = append(errs, field.Invalid(
							field.NewPath("emitter", "agentgateway", "AgentgatewayPolicy"),
							polSourceIngressName,
							fmt.Sprintf("policy only applies to a subset of backendRefs within the HTTPRoute (%d/%d); "+
								"agentgateway requires full policy coverage because it does not support attaching policies "+
								"via HTTPRoute filter ExtensionRef", covered, total),
						))
						delete(agentgatewayPolicies, polSourceIngressName)
					}
				}
			}
		}

		// Write back the mutated HTTPRouteContext into the IR.
		ir.HTTPRoutes[httpRouteKey] = httpRouteContext

		// Update gatewayResources with modified HTTPRoute
		gatewayResources.HTTPRoutes[httpRouteKey] = httpRouteContext.HTTPRoute
	}

	// Split HTTPRoutes that have SSL redirect enabled
	for httpRouteKey := range routesToSplitForSSLRedirect {
		httpRouteContext, exists := ir.HTTPRoutes[httpRouteKey]
		if !exists {
			continue
		}

		// Get the Gateway for this HTTPRoute
		var gatewayCtx *emitterir.GatewayContext
		if len(httpRouteContext.Spec.ParentRefs) > 0 {
			parentRef := httpRouteContext.Spec.ParentRefs[0]
			gatewayNamespace := httpRouteKey.Namespace
			if parentRef.Namespace != nil {
				gatewayNamespace = string(*parentRef.Namespace)
			}
			gatewayName := string(parentRef.Name)
			if gatewayName != "" {
				gatewayKey := types.NamespacedName{
					Namespace: gatewayNamespace,
					Name:      gatewayName,
				}
				if gw, ok := ir.Gateways[gatewayKey]; ok {
					gatewayCtx = &gw
				}
			}
		}

		if gatewayCtx == nil {
			continue
		}

		// Split the route
		httpRedirectRoute, httpsBackendRoute, success := splitHTTPRouteForSSLRedirect(
			httpRouteContext,
			httpRouteKey,
			gatewayCtx,
		)
		if !success || httpRedirectRoute == nil {
			continue
		}

		// Decide which route should receive any HTTPRoute-scoped AgentgatewayPolicies.
		// Prefer the HTTPS backend route (the route that still has backendRefs).
		// If no HTTPS route was created (e.g. no HTTPS listener), fall back to the HTTP redirect route.
		newPolicyRouteName := httpRedirectRoute.Name
		if httpsBackendRoute != nil {
			newPolicyRouteName = httpsBackendRoute.Name
		}

		// Retarget existing HTTPRoute-scoped policies that pointed at the original route.
		for _, ap := range agentgatewayPolicies {
			if ap == nil {
				continue
			}
			for i := range ap.Spec.TargetRefs {
				tr := &ap.Spec.TargetRefs[i]
				// Only retarget policies attached to this HTTPRoute.
				if tr.LocalPolicyTargetReference.Group == gatewayv1.Group("gateway.networking.k8s.io") &&
					tr.LocalPolicyTargetReference.Kind == gatewayv1.Kind("HTTPRoute") &&
					string(tr.LocalPolicyTargetReference.Name) == httpRouteKey.Name {
					tr.LocalPolicyTargetReference.Name = gatewayv1.ObjectName(newPolicyRouteName)
				}
			}
		}

		// Remove the original route
		delete(ir.HTTPRoutes, httpRouteKey)
		delete(gatewayResources.HTTPRoutes, httpRouteKey)

		// Add the HTTP redirect route
		httpRedirectKey := types.NamespacedName{
			Namespace: httpRedirectRoute.Namespace,
			Name:      httpRedirectRoute.Name,
		}
		ir.HTTPRoutes[httpRedirectKey] = *httpRedirectRoute
		gatewayResources.HTTPRoutes[httpRedirectKey] = httpRedirectRoute.HTTPRoute

		// Add the HTTPS backend route if it was created
		if httpsBackendRoute != nil {
			httpsBackendKey := types.NamespacedName{
				Namespace: httpsBackendRoute.Namespace,
				Name:      httpsBackendRoute.Name,
			}
			ir.HTTPRoutes[httpsBackendKey] = *httpsBackendRoute
			gatewayResources.HTTPRoutes[httpsBackendKey] = httpsBackendRoute.HTTPRoute
		}
	}

	// Collect AgentgatewayPolicies
	for _, ap := range agentgatewayPolicies {
		agentgatewayObjs = append(agentgatewayObjs, ap)
	}

	// Collect backend-scoped policies (Service-targeted).
	for _, ap := range backendPolicies {
		agentgatewayObjs = append(agentgatewayObjs, ap)
	}

	// Collect AgentgatewayBackends generated by service-upstream.
	for _, be := range agentgatewayBackends {
		agentgatewayObjs = append(agentgatewayObjs, be)
	}

	// Sort by Kind, then Namespace, then Name to make output deterministic for testing.
	sort.SliceStable(agentgatewayObjs, func(i, j int) bool {
		oi, oj := agentgatewayObjs[i], agentgatewayObjs[j]

		gvki := oi.GetObjectKind().GroupVersionKind()
		gvkj := oj.GetObjectKind().GroupVersionKind()

		ki, kj := gvki.Kind, gvkj.Kind
		if ki != kj {
			return ki < kj
		}

		nsi, nsj := oi.GetNamespace(), oj.GetNamespace()
		if nsi != nsj {
			return nsi < nsj
		}

		return oi.GetName() < oj.GetName()
	})

	// Convert agentgateway objects to unstructured and add to GatewayExtensions
	for _, obj := range agentgatewayObjs {
		u, err := toUnstructured(obj)
		if err != nil {
			errs = append(errs, field.InternalError(field.NewPath("agentgateway"), err))
			continue
		}
		gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *u)
	}

	return gatewayResources, errs
}
