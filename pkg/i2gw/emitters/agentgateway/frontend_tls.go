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
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
)

// applyFrontendTLSPolicy projects frontend TLS listener settings into
// AgentgatewayPolicy.spec.frontend.tls.
func applyFrontendTLSPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.FrontendTLS == nil {
		return false
	}
	if pol.FrontendTLS.HandshakeTimeout == nil && len(pol.FrontendTLS.ALPNProtocols) == 0 {
		return false
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Frontend == nil {
		agp.Spec.Frontend = &agentgatewayv1alpha1.Frontend{}
	}
	if agp.Spec.Frontend.TLS == nil {
		agp.Spec.Frontend.TLS = &agentgatewayv1alpha1.FrontendTLS{}
	}

	if pol.FrontendTLS.HandshakeTimeout != nil {
		agp.Spec.Frontend.TLS.HandshakeTimeout = pol.FrontendTLS.HandshakeTimeout
	}
	if len(pol.FrontendTLS.ALPNProtocols) > 0 {
		alpn := make([]agentgatewayv1alpha1.TinyString, 0, len(pol.FrontendTLS.ALPNProtocols))
		for _, value := range pol.FrontendTLS.ALPNProtocols {
			alpn = append(alpn, agentgatewayv1alpha1.TinyString(value))
		}
		agp.Spec.Frontend.TLS.AlpnProtocols = &alpn
	}

	ap[ingressName] = agp
	return true
}
