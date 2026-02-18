/*
Copyright 2026 The Kubernetes Authors.

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

package kgateway

import (
	"fmt"

	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/kgateway"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
)

type BuilderMap struct {
	TrafficPolicies map[types.NamespacedName]*kgateway.TrafficPolicy
}

func NewBuilderMap() *BuilderMap {
	return &BuilderMap{
		TrafficPolicies: make(map[types.NamespacedName]*kgateway.TrafficPolicy),
	}
}

func (e *Emitter) getOrBuildTrafficPolicy(ctx emitterir.HTTPRouteContext, sectionName *gatewayv1.SectionName, ruleIdx int) *kgateway.TrafficPolicy {
	name := fmt.Sprintf("%s-%d", ctx.Name, ruleIdx)
	if ruleIdx == RouteRuleAllIndex {
		name = ctx.Name
	}
	key := types.NamespacedName{
		Name:      name,
		Namespace: ctx.Namespace,
	}
	policy, exist := e.builderMap.TrafficPolicies[key]
	if exist {
		return policy
	}

	trafficPolicy := &kgateway.TrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ctx.Namespace,
		},
		Spec: kgateway.TrafficPolicySpec{
			TargetRefs: []shared.LocalPolicyTargetReferenceWithSectionName{
				{
					LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
						Group: gatewayv1.Group("gateway.networking.k8s.io"),
						Kind:  gatewayv1.Kind("HTTPRoute"),
						Name:  gatewayv1.ObjectName(ctx.Name),
					},
					SectionName: sectionName,
				},
			},
		},
	}
	trafficPolicy.SetGroupVersionKind(TrafficPolicyGVK)

	e.builderMap.TrafficPolicies[key] = trafficPolicy
	return trafficPolicy
}
