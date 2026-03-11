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

func TestApplyFrontendHTTPPolicyTargetsGateway(t *testing.T) {
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
	http1MaxHeaders := int32(120)
	http1IdleTimeout := metav1.Duration{Duration: 45 * time.Second}
	http2WindowSize := int32(65535)
	http2ConnectionWindowSize := int32(131070)
	http2FrameSize := int32(16384)
	http2KeepaliveInterval := metav1.Duration{Duration: 30 * time.Second}
	http2KeepaliveTimeout := metav1.Duration{Duration: 10 * time.Second}
	policy := emitterir.Policy{
		FrontendHTTP: &emitterir.FrontendHTTPPolicy{
			HTTP1MaxHeaders:           &http1MaxHeaders,
			HTTP1IdleTimeout:          &http1IdleTimeout,
			HTTP2WindowSize:           &http2WindowSize,
			HTTP2ConnectionWindowSize: &http2ConnectionWindowSize,
			HTTP2FrameSize:            &http2FrameSize,
			HTTP2KeepaliveInterval:    &http2KeepaliveInterval,
			HTTP2KeepaliveTimeout:     &http2KeepaliveTimeout,
		},
	}

	touched, err := applyFrontendHTTPPolicy(policy, "ingress-frontend-http", httpRoute, "default", policies, sources)
	if err != nil {
		t.Fatalf("applyFrontendHTTPPolicy returned error: %v", err)
	}
	if !touched {
		t.Fatal("expected frontend HTTP policy to be emitted")
	}

	key := types.NamespacedName{Namespace: "default", Name: "nginx"}
	got := policies[key]
	if got == nil {
		t.Fatalf("expected frontend HTTP policy for %v", key)
	}
	if got.Name != "nginx-frontend-http" {
		t.Fatalf("expected Gateway-scoped frontend HTTP policy name %q, got %q", "nginx-frontend-http", got.Name)
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
		t.Fatalf("expected Gateway frontend HTTP policy to omit sectionName, got %q", *targetRef.SectionName)
	}
}

func TestApplyFrontendHTTPPolicyRejectsConflictingGatewaySettings(t *testing.T) {
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
	firstMaxHeaders := int32(120)
	first := emitterir.Policy{
		FrontendHTTP: &emitterir.FrontendHTTPPolicy{
			HTTP1MaxHeaders: &firstMaxHeaders,
		},
	}
	if _, err := applyFrontendHTTPPolicy(first, "ingress-a", httpRoute, "default", policies, sources); err != nil {
		t.Fatalf("unexpected error creating first frontend HTTP policy: %v", err)
	}

	conflictingMaxHeaders := int32(121)
	conflicting := emitterir.Policy{
		FrontendHTTP: &emitterir.FrontendHTTPPolicy{
			HTTP1MaxHeaders: &conflictingMaxHeaders,
		},
	}
	if _, err := applyFrontendHTTPPolicy(conflicting, "ingress-b", httpRoute, "default", policies, sources); err == nil {
		t.Fatal("expected conflicting frontend HTTP settings to return an error")
	}
}
