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

package agentgateway

import (
	"fmt"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyFrontendTLSPolicy projects frontend TLS listener settings into
// AgentgatewayPolicy.spec.frontend.tls.
//
// Agentgateway validates frontend policies only when they target the Gateway
// resource directly, so these settings are tracked separately from the
// HTTPRoute-scoped policy map used by traffic features.
func applyFrontendTLSPolicy(
	pol emitterir.Policy,
	ingressName string,
	httpRoute gatewayv1.HTTPRoute,
	defaultNamespace string,
	ap map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy,
	sources map[types.NamespacedName]string,
) (bool, *field.Error) {
	if pol.FrontendTLS == nil {
		return false, nil
	}
	if pol.FrontendTLS.HandshakeTimeout == nil && len(pol.FrontendTLS.ALPNProtocols) == 0 {
		return false, nil
	}

	gatewayKey, ok := gatewayParentKeyForHTTPRoute(httpRoute, defaultNamespace)
	if !ok {
		return false, field.Invalid(
			field.NewPath("emitter", "agentgateway", "AgentgatewayPolicy", "frontend", "tls"),
			ingressName,
			"frontend TLS policies require a parent Gateway target",
		)
	}

	if existing, ok := ap[gatewayKey]; ok {
		if !frontendTLSPoliciesEqual(existing.Spec.Frontend.TLS, pol.FrontendTLS) {
			return false, field.Invalid(
				field.NewPath("emitter", "agentgateway", "AgentgatewayPolicy", "frontend", "tls"),
				ingressName,
				fmt.Sprintf(
					"frontend TLS settings conflict on Gateway %s/%s with source Ingress %q; agentgateway only supports Gateway-scoped frontend TLS policies",
					gatewayKey.Namespace,
					gatewayKey.Name,
					sources[gatewayKey],
				),
			)
		}
		return true, nil
	}

	alpn := make([]agentgatewayv1alpha1.TinyString, 0, len(pol.FrontendTLS.ALPNProtocols))
	for _, value := range pol.FrontendTLS.ALPNProtocols {
		alpn = append(alpn, agentgatewayv1alpha1.TinyString(value))
	}

	ap[gatewayKey] = &agentgatewayv1alpha1.AgentgatewayPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-frontend-tls", gatewayKey.Name),
			Namespace: gatewayKey.Namespace,
		},
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
			Frontend: &agentgatewayv1alpha1.Frontend{
				TLS: &agentgatewayv1alpha1.FrontendTLS{
					HandshakeTimeout: pol.FrontendTLS.HandshakeTimeout,
					AlpnProtocols:    &alpn,
				},
			},
			TargetRefs: []shared.LocalPolicyTargetReferenceWithSectionName{{
				LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
					Group: gatewayv1.Group("gateway.networking.k8s.io"),
					Kind:  gatewayv1.Kind("Gateway"),
					Name:  gatewayv1.ObjectName(gatewayKey.Name),
				},
			}},
		},
	}
	ap[gatewayKey].SetGroupVersionKind(AgentgatewayPolicyGVK)
	sources[gatewayKey] = ingressName

	return true, nil
}

func frontendTLSPoliciesEqual(
	existing *agentgatewayv1alpha1.FrontendTLS,
	desired *emitterir.FrontendTLSPolicy,
) bool {
	if existing == nil || desired == nil {
		return existing == nil && desired == nil
	}

	switch {
	case existing.HandshakeTimeout == nil && desired.HandshakeTimeout != nil:
		return false
	case existing.HandshakeTimeout != nil && desired.HandshakeTimeout == nil:
		return false
	case existing.HandshakeTimeout != nil && desired.HandshakeTimeout != nil &&
		existing.HandshakeTimeout.Duration != desired.HandshakeTimeout.Duration:
		return false
	}

	existingALPN := []agentgatewayv1alpha1.TinyString{}
	if existing.AlpnProtocols != nil {
		existingALPN = *existing.AlpnProtocols
	}
	if len(existingALPN) != len(desired.ALPNProtocols) {
		return false
	}
	for i, protocol := range desired.ALPNProtocols {
		if string(existingALPN[i]) != protocol {
			return false
		}
	}

	return true
}
