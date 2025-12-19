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

package envoygateway_emitter

import (
	egv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func buildSecurityPolicy(targetNN types.NamespacedName, sectionName *string) *egv1a1.SecurityPolicy {
	securityPolicy := &egv1a1.SecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: targetNN.Namespace,
			Name:      targetNN.Name,
		},
		Spec: egv1a1.SecurityPolicySpec{
			PolicyTargetReferences: egv1a1.PolicyTargetReferences{
				TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
							Group: "gateway.networking.k8s.io",
							Kind:  "HTTPRoute",
							Name:  gatewayv1.ObjectName(targetNN.Name),
						},
					},
				},
			},
		},
	}
	securityPolicy.SetGroupVersionKind(securityPolicyGVK)
	if sectionName != nil {
		securityPolicy.Spec.PolicyTargetReferences.TargetRefs[0].SectionName = ptr.To(gatewayv1.SectionName(*sectionName))
	}
	return securityPolicy
}

func buildSecurityPolicyExtAuth(extAuthConfig *emitterir.ExternalAuthConfig) *egv1a1.ExtAuth {
	extAuth := &egv1a1.ExtAuth{}

	if extAuthConfig.Protocol == gatewayv1.HTTPRouteExternalAuthHTTPProtocol {
		extAuth.HTTP = &egv1a1.HTTPExtAuthService{
			BackendCluster: egv1a1.BackendCluster{
				BackendRefs: []egv1a1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Group:     extAuthConfig.BackendObjectReference.Group,
							Kind:      extAuthConfig.BackendObjectReference.Kind,
							Name:      extAuthConfig.BackendObjectReference.Name,
							Namespace: extAuthConfig.BackendObjectReference.Namespace,
							Port:      extAuthConfig.BackendObjectReference.Port,
						},
					},
				},
			},
			Path:             ptr.To(extAuthConfig.Path),
			HeadersToBackend: extAuthConfig.AllowedResponseHeaders,
		}
	}
	return extAuth
}
