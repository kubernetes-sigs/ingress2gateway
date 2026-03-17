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
	"testing"
	"time"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestApplyFrontendTLSPolicyTargetsGateway(t *testing.T) {
	policies := map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy{}
	sources := map[types.NamespacedName]string{}
	httpRoute := gatewayv1.HTTPRoute{
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name: gatewayv1.ObjectName("nginx"),
				}},
			},
		},
	}
	policy := emitterir.Policy{
		FrontendTLS: &emitterir.FrontendTLSPolicy{
			HandshakeTimeout: &metav1.Duration{Duration: 20 * time.Second},
			ALPNProtocols:    []string{"h2", "http/1.1"},
		},
	}

	touched, err := applyFrontendTLSPolicy(policy, "ingress-frontend-tls", httpRoute, "default", policies, sources)
	if err != nil {
		t.Fatalf("applyFrontendTLSPolicy returned error: %v", err)
	}
	if !touched {
		t.Fatal("expected frontend TLS policy to be emitted")
	}

	key := types.NamespacedName{Namespace: "default", Name: "nginx"}
	got := policies[key]
	if got == nil {
		t.Fatalf("expected frontend TLS policy for %v", key)
	}
	if got.Name != "nginx-frontend-tls" {
		t.Fatalf("expected Gateway-scoped frontend TLS policy name %q, got %q", "nginx-frontend-tls", got.Name)
	}
	if len(got.Spec.TargetRefs) != 1 {
		t.Fatalf("expected a single targetRef, got %d", len(got.Spec.TargetRefs))
	}
	targetRef := got.Spec.TargetRefs[0]
	if targetRef.Kind != gatewayv1.Kind("Gateway") {
		t.Fatalf("expected Gateway target, got %q", targetRef.Kind)
	}
	if targetRef.Name != gatewayv1.ObjectName("nginx") {
		t.Fatalf("expected Gateway target name %q, got %q", "nginx", targetRef.Name)
	}
	if targetRef.SectionName != nil {
		t.Fatalf("expected Gateway frontend TLS policy to omit sectionName, got %q", *targetRef.SectionName)
	}
}

func TestApplyFrontendTLSPolicyRejectsConflictingGatewaySettings(t *testing.T) {
	policies := map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy{}
	sources := map[types.NamespacedName]string{}
	httpRoute := gatewayv1.HTTPRoute{
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name: gatewayv1.ObjectName("nginx"),
				}},
			},
		},
	}

	first := emitterir.Policy{
		FrontendTLS: &emitterir.FrontendTLSPolicy{
			HandshakeTimeout: &metav1.Duration{Duration: 20 * time.Second},
			ALPNProtocols:    []string{"h2", "http/1.1"},
		},
	}
	if _, err := applyFrontendTLSPolicy(first, "ingress-a", httpRoute, "default", policies, sources); err != nil {
		t.Fatalf("unexpected error creating first frontend TLS policy: %v", err)
	}

	conflicting := emitterir.Policy{
		FrontendTLS: &emitterir.FrontendTLSPolicy{
			HandshakeTimeout: &metav1.Duration{Duration: 30 * time.Second},
			ALPNProtocols:    []string{"h2", "http/1.1"},
		},
	}
	if _, err := applyFrontendTLSPolicy(conflicting, "ingress-b", httpRoute, "default", policies, sources); err == nil {
		t.Fatal("expected conflicting frontend TLS settings to return an error")
	}
}
