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

	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyBackendProtocolPolicy projects ingress-nginx backend-protocol into backend HTTP version policies.
//
// Current semantics:
//   - Only BackendProtocolGRPC is produced by the provider IR and maps to HTTP2.
//   - If a backendRef points to AgentgatewayBackend (e.g. service-upstream rewrite), target that backend.
//   - Otherwise, for core Service backendRefs, target the Service.
func applyBackendProtocolPolicy(
	pol emitterir.Policy,
	httpRouteKey types.NamespacedName,
	httpRouteCtx emitterir.HTTPRouteContext,
	backendPolicies map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy,
) {
	if pol.BackendProtocol == nil || *pol.BackendProtocol != emitterir.BackendProtocolGRPC {
		return
	}

	for _, idx := range pol.RuleBackendSources {
		if idx.Rule >= len(httpRouteCtx.Spec.Rules) {
			continue
		}
		rule := httpRouteCtx.Spec.Rules[idx.Rule]
		if idx.Backend >= len(rule.BackendRefs) {
			continue
		}

		br := rule.BackendRefs[idx.Backend]
		refName := string(br.BackendRef.Name)
		if refName == "" {
			continue
		}

		targetGroup := gwv1.Group("")
		targetKind := gwv1.Kind("Service")

		switch {
		case br.BackendRef.Group != nil && *br.BackendRef.Group == gwv1.Group(AgentgatewayBackendGVK.Group) &&
			br.BackendRef.Kind != nil && *br.BackendRef.Kind == gwv1.Kind(AgentgatewayBackendGVK.Kind):
			targetGroup = gwv1.Group(AgentgatewayBackendGVK.Group)
			targetKind = gwv1.Kind(AgentgatewayBackendGVK.Kind)
		case (br.BackendRef.Group == nil || *br.BackendRef.Group == "") &&
			(br.BackendRef.Kind == nil || *br.BackendRef.Kind == "Service"):
			// Default path: target core Service backends.
		default:
			continue
		}

		targetKey := types.NamespacedName{Namespace: httpRouteKey.Namespace, Name: refName}
		ap, exists := backendPolicies[targetKey]
		if !exists {
			ap = &agentgatewayv1alpha1.AgentgatewayPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      refName + "-backend-http-version",
					Namespace: httpRouteKey.Namespace,
				},
				Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
					TargetRefs: []shared.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
								Group: targetGroup,
								Kind:  targetKind,
								Name:  gwv1.ObjectName(refName),
							},
						},
					},
				},
			}
			ap.SetGroupVersionKind(AgentgatewayPolicyGVK)
			backendPolicies[targetKey] = ap
		}

		if ap.Spec.Backend == nil {
			ap.Spec.Backend = &agentgatewayv1alpha1.BackendFull{}
		}
		if ap.Spec.Backend.HTTP == nil {
			ap.Spec.Backend.HTTP = &agentgatewayv1alpha1.BackendHTTP{}
		}
		ap.Spec.Backend.HTTP.Version = ptrHTTPVersion(agentgatewayv1alpha1.HTTPVersion2)
	}
}

func ptrHTTPVersion(v agentgatewayv1alpha1.HTTPVersion) *agentgatewayv1alpha1.HTTPVersion {
	return &v
}
