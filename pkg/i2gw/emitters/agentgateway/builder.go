/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agentgateway

import (
	"fmt"

	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/shared"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type BuilderMap struct {
	Policies map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy
}

func NewBuilderMap() *BuilderMap {
	return &BuilderMap{Policies: make(map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy)}
}

func (e *Emitter) getOrBuildPolicy(ctx emitterir.HTTPRouteContext, sectionName *gatewayv1.SectionName, ruleIdx int) *agentgatewayv1alpha1.AgentgatewayPolicy {
	objName := gatewayv1.ObjectName(ctx.Name)
	name := objName
	if ruleIdx != RouteRuleAllIndex {
		name = gatewayv1.ObjectName(fmt.Sprintf("%s-%d", ctx.Name, ruleIdx))
	}

	key := types.NamespacedName{Namespace: ctx.Namespace, Name: string(name)}
	if policy, exists := e.builderMap.Policies[key]; exists {
		return policy
	}

	policy := &agentgatewayv1alpha1.AgentgatewayPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(name),
			Namespace: ctx.Namespace,
		},
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
			TargetRefs: []shared.LocalPolicyTargetReferenceWithSectionName{{
				LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
					Group: gatewayv1.Group("gateway.networking.k8s.io"),
					Kind:  gatewayv1.Kind("HTTPRoute"),
					Name:  gatewayv1.ObjectName(ctx.Name),
				},
				SectionName: sectionName,
			}},
		},
	}
	policy.SetGroupVersionKind(AgentgatewayPolicyGVK)
	e.builderMap.Policies[key] = policy

	return policy
}
