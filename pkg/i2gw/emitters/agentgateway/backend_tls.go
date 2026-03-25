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
	"strings"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	"github.com/agentgateway/agentgateway/controller/api/v1alpha1/shared"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyBackendTLSPolicy projects the BackendTLS IR policy into one or more AgentgatewayPolicies.
//
// Semantics:
//   - We create at most one AgentgatewayPolicy per Service.
//   - That policy's Spec.Backend.TLS is configured with SNI hostname, client certificate secret,
//     and verification settings from Policy.BackendTLS.
//   - TargetRefs are populated with each covered core Service backend (based on RuleBackendSources).
//
// Notes:
//   - The ingress-nginx IR provides a single SecretName; we map this to mtlsCertificateRef when Verify=true.
//   - Agentgateway backend TLS uses:
//   - mtlsCertificateRef: Secret containing tls.crt/tls.key (and optional ca.cert)
//   - caCertificateRefs: ConfigMap refs (not used here)
//   - insecureSkipVerify: enum (All|Hostname)
func applyBackendTLSPolicy(
	pol emitterir.Policy,
	httpRouteKey types.NamespacedName,
	httpRouteCtx emitterir.HTTPRouteContext,
	backendTLSPolicies map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.BackendTLS == nil {
		return false
	}

	backendTLS := pol.BackendTLS

	// Parse secret name (format: "namespace/secretName" or just "secretName").
	// AgentgatewayPolicy references Secrets by name (same namespace).
	secretName := backendTLS.SecretName
	if parts := strings.SplitN(backendTLS.SecretName, "/", 2); len(parts) == 2 {
		secretName = parts[1]
	}

	changed := false

	for _, idx := range pol.RuleBackendSources {
		if idx.Rule >= len(httpRouteCtx.Spec.Rules) {
			continue
		}
		rule := httpRouteCtx.Spec.Rules[idx.Rule]
		if idx.Backend >= len(rule.BackendRefs) {
			continue
		}

		br := rule.BackendRefs[idx.Backend]

		// Only apply to core Services (same semantics as kgateway emitter).
		if br.BackendRef.Group != nil && *br.BackendRef.Group != "" {
			continue
		}
		if br.BackendRef.Kind != nil && *br.BackendRef.Kind != "" && *br.BackendRef.Kind != "Service" {
			continue
		}

		svcName := string(br.BackendRef.Name)
		if svcName == "" {
			continue
		}

		svcKey := types.NamespacedName{
			Namespace: httpRouteKey.Namespace,
			Name:      svcName,
		}

		ap, ok := backendTLSPolicies[svcKey]
		if !ok {
			ap = &agentgatewayv1alpha1.AgentgatewayPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      svcName + "-backend-tls",
					Namespace: httpRouteKey.Namespace,
				},
				Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
					TargetRefs: []shared.LocalPolicyTargetReferenceWithSectionName{{
						LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  gatewayv1.ObjectName(svcName),
						},
					}},
				},
			}
			// Ensure GVK is set for deterministic unstructured conversion.
			ap.SetGroupVersionKind(AgentgatewayPolicyGVK)
			backendTLSPolicies[svcKey] = ap
		}

		if ap.Spec.Backend == nil {
			ap.Spec.Backend = &agentgatewayv1alpha1.BackendFull{}
		}
		if ap.Spec.Backend.TLS == nil {
			ap.Spec.Backend.TLS = &agentgatewayv1alpha1.BackendTLS{}
		}

		// Set SNI hostname if specified.
		if backendTLS.Hostname != "" {
			ap.Spec.Backend.TLS.Sni = ptr.To(backendTLS.Hostname)
		}

		// Verification mapping:
		// - Verify=false => skip verification entirely (closest match: InsecureTLSModeAll).
		// - Verify=true  => use system roots unless a Secret is provided; if Secret provided, use it for mTLS.
		if !backendTLS.Verify {
			ap.Spec.Backend.TLS.InsecureSkipVerify = ptr.To(agentgatewayv1alpha1.InsecureTLSModeAll)
			// Do not set mTLS secret when verification is disabled (matches kgateway emitter semantics).
			ap.Spec.Backend.TLS.MtlsCertificateRef = nil
		} else if secretName != "" {
			ap.Spec.Backend.TLS.MtlsCertificateRef = []corev1.LocalObjectReference{{
				Name: secretName,
			}}
		}

		changed = true
	}

	return changed
}
