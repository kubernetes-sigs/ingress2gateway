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

package envoygateway_emitter

import (
	"fmt"

	egapiv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
)

type BuilderMap struct {
	BackendTrafficPolicies map[types.NamespacedName]*egapiv1a1.BackendTrafficPolicy
	SecurityPolicies       map[types.NamespacedName]*egapiv1a1.SecurityPolicy
}

func NewBuilderMap() *BuilderMap {
	return &BuilderMap{
		BackendTrafficPolicies: make(map[types.NamespacedName]*egapiv1a1.BackendTrafficPolicy),
		SecurityPolicies:       make(map[types.NamespacedName]*egapiv1a1.SecurityPolicy),
	}
}

func (e *Emitter) getOrBuildBackendTrafficPolicy(ctx emitterir.HTTPRouteContext, sectionName *gwapiv1.SectionName, ruleIdx int) *egapiv1a1.BackendTrafficPolicy {
	name := fmt.Sprintf("%s-%d", ctx.Name, ruleIdx)
	if ruleIdx == RouteRuleAllIndex {
		name = ctx.Name
	}
	key := types.NamespacedName{
		Name:      name,
		Namespace: ctx.Namespace,
	}
	policy, exist := e.builderMap.BackendTrafficPolicies[key]
	if exist {
		return policy
	}

	backendTrafficPolicy := &egapiv1a1.BackendTrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ctx.Namespace,
		},
		Spec: egapiv1a1.BackendTrafficPolicySpec{
			PolicyTargetReferences: egapiv1a1.PolicyTargetReferences{
				TargetRefs: []gwapiv1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: gwapiv1.LocalPolicyTargetReference{
							Group: gwapiv1.Group(HTTPRouteGVK.Group),
							Kind:  gwapiv1.Kind(HTTPRouteGVK.Kind),
							Name:  gwapiv1.ObjectName(ctx.Name),
						},
						SectionName: sectionName,
					},
				},
			},
		},
	}
	backendTrafficPolicy.SetGroupVersionKind(BackendTrafficPolicyGVK)

	e.builderMap.BackendTrafficPolicies[key] = backendTrafficPolicy
	return backendTrafficPolicy
}

func (e *Emitter) getOrBuildSecurityPolicy(ctx emitterir.HTTPRouteContext, sectionName *gwapiv1.SectionName, ruleIdx int) *egapiv1a1.SecurityPolicy {
	name := fmt.Sprintf("%s-%d", ctx.Name, ruleIdx)
	if ruleIdx == RouteRuleAllIndex {
		name = ctx.Name
	}
	key := types.NamespacedName{
		Name:      name,
		Namespace: ctx.Namespace,
	}
	policy, exist := e.builderMap.SecurityPolicies[key]
	if exist {
		return policy
	}

	securityPolicy := &egapiv1a1.SecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ctx.Namespace,
		},
		Spec: egapiv1a1.SecurityPolicySpec{
			PolicyTargetReferences: egapiv1a1.PolicyTargetReferences{
				TargetRefs: []gwapiv1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: gwapiv1.LocalPolicyTargetReference{
							Group: gwapiv1.Group(HTTPRouteGVK.Group),
							Kind:  gwapiv1.Kind(HTTPRouteGVK.Kind),
							Name:  gwapiv1.ObjectName(ctx.Name),
						},
						SectionName: sectionName,
					},
				},
			},
		},
	}
	securityPolicy.SetGroupVersionKind(SecurityPolicyGVK)

	e.builderMap.SecurityPolicies[key] = securityPolicy
	return securityPolicy
}
