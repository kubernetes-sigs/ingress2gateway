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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// uniquePolicyIndices returns a slice of PolicyIndex values with duplicates
// removed. Uniqueness is defined by the (Rule, Backend) pair.
func uniquePolicyIndices(indices []emitterir.PolicyIndex) []emitterir.PolicyIndex {
	if len(indices) == 0 {
		return indices
	}

	seen := make(map[emitterir.PolicyIndex]struct{}, len(indices))
	out := make([]emitterir.PolicyIndex, 0, len(indices))

	for _, idx := range indices {
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, idx)
	}
	return out
}

// ensureAgentgatewayPolicy returns the AgentgatewayPolicy for the given ingressName,
// creating and initializing it if needed.
func ensureAgentgatewayPolicy(
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
	ingressName, namespace string,
) *agentgatewayv1alpha1.AgentgatewayPolicy {
	if existing, ok := ap[ingressName]; ok {
		return existing
	}

	newAP := &agentgatewayv1alpha1.AgentgatewayPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{},
	}
	newAP.SetGroupVersionKind(AgentgatewayPolicyGVK)

	ap[ingressName] = newAP
	return newAP
}

func numRules(hr gatewayv1.HTTPRoute) int {
	n := 0
	for _, r := range hr.Spec.Rules {
		n += len(r.BackendRefs)
	}
	return n
}

// toUnstructured converts a runtime.Object to unstructured.Unstructured
func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstructuredObj}, nil
}
