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
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	"github.com/agentgateway/agentgateway/controller/api/v1alpha1/shared"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyRequestTimeoutPolicy projects HTTP request timeout-related Policy IR into an AgentgatewayPolicy,
// returning true if it modified/created an AgentgatewayPolicy for this ingress.
//
// AgentgatewayPolicy exposes a single request timeout at traffic.timeouts.request.
// NGINX has separate proxy-send-timeout and proxy-read-timeout knobs; we conservatively
// choose the larger of the two when both are set.
func applyRequestTimeoutPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.ProxyReadTimeout == nil && pol.ProxySendTimeout == nil {
		return false
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Traffic == nil {
		agp.Spec.Traffic = &agentgatewayv1alpha1.Traffic{}
	}
	if agp.Spec.Traffic.Timeouts == nil {
		agp.Spec.Traffic.Timeouts = &agentgatewayv1alpha1.Timeouts{}
	}

	// Pick the most permissive timeout to avoid unexpectedly truncating requests.
	timeout := pol.ProxySendTimeout
	if timeout == nil || (pol.ProxyReadTimeout != nil && pol.ProxyReadTimeout.Duration > timeout.Duration) {
		timeout = pol.ProxyReadTimeout
	}

	agp.Spec.Traffic.Timeouts.Request = timeout
	ap[ingressName] = agp
	return true
}

// effectiveRequestTimeout returns the request timeout that applyRequestTimeoutPolicy
// would set, without creating/modifying objects.
func effectiveRequestTimeout(pol emitterir.Policy) *metav1.Duration {
	if pol.ProxyReadTimeout == nil && pol.ProxySendTimeout == nil {
		return nil
	}
	// Same logic as applyRequestTimeoutPolicy: pick the larger (most permissive).
	t := pol.ProxySendTimeout
	if t == nil || (pol.ProxyReadTimeout != nil && pol.ProxyReadTimeout.Duration > t.Duration) {
		t = pol.ProxyReadTimeout
	}
	return t
}

// applyProxyConnectTimeoutPolicy projects proxy-connect-timeout into per-Service backend TCP connectTimeout policies.
//
// Semantics:
//   - Emits at most one AgentgatewayPolicy per Service (ns/name) that this Policy covers.
//   - Sets spec.backend.tcp.connectTimeout when it can have effect.
//   - If the ingress-wide request timeout (traffic.timeouts.request) is <= connectTimeout,
//     we skip projecting connectTimeout because the request timeout will fire first.
//   - Across contributors for the same Service, "lowest connect timeout wins".
func applyProxyConnectTimeoutPolicy(
	pol emitterir.Policy,
	ingressName string,
	httpRouteKey types.NamespacedName,
	httpRouteCtx emitterir.HTTPRouteContext,
	trafficPolicies map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
	backendPolicies map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.ProxyConnectTimeout == nil {
		return false
	}

	// Prefer the already-computed request timeout (set by applyRequestTimeoutPolicy),
	// falling back to the same calculation if it wasn’t set/processed yet.
	req := func() *metav1.Duration {
		if agp := trafficPolicies[ingressName]; agp != nil &&
			agp.Spec.Traffic != nil && agp.Spec.Traffic.Timeouts != nil &&
			agp.Spec.Traffic.Timeouts.Request != nil {
			return agp.Spec.Traffic.Timeouts.Request
		}
		return effectiveRequestTimeout(pol)
	}()

	for _, idx := range pol.RuleBackendSources {
		if idx.Rule >= len(httpRouteCtx.Spec.Rules) {
			continue
		}
		rule := httpRouteCtx.Spec.Rules[idx.Rule]
		if idx.Backend >= len(rule.BackendRefs) {
			continue
		}

		br := rule.BackendRefs[idx.Backend]
		if br.BackendRef.Group != nil && *br.BackendRef.Group != "" {
			continue
		}
		if br.BackendRef.Kind != nil && *br.BackendRef.Kind != "Service" {
			continue
		}

		svcName := string(br.BackendRef.Name)
		if svcName == "" {
			continue
		}

		// If request timeout is already <= connect timeout, connect timeout can’t take effect.
		if req != nil && pol.ProxyConnectTimeout.Duration >= req.Duration {
			continue
		}

		svcKey := types.NamespacedName{Namespace: httpRouteKey.Namespace, Name: svcName}
		ap, exists := backendPolicies[svcKey]
		if !exists {
			ap = &agentgatewayv1alpha1.AgentgatewayPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      svcName + "-backend-connect-timeout",
					Namespace: httpRouteKey.Namespace,
				},
				Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
					TargetRefs: []shared.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
								Group: gwv1.Group(""),
								Kind:  gwv1.Kind("Service"),
								Name:  gwv1.ObjectName(svcName),
							},
						},
					},
				},
			}
			ap.SetGroupVersionKind(AgentgatewayPolicyGVK)
			backendPolicies[svcKey] = ap
		}

		if ap.Spec.Backend == nil {
			ap.Spec.Backend = &agentgatewayv1alpha1.BackendFull{}
		}
		if ap.Spec.Backend.TCP == nil {
			ap.Spec.Backend.TCP = &agentgatewayv1alpha1.BackendTCP{}
		}

		// The lowest connect timeout wins across contributors for the same Service policy.
		if ap.Spec.Backend.TCP.ConnectTimeout == nil ||
			pol.ProxyConnectTimeout.Duration < ap.Spec.Backend.TCP.ConnectTimeout.Duration {
			ap.Spec.Backend.TCP.ConnectTimeout = pol.ProxyConnectTimeout
		}
	}

	return true
}
