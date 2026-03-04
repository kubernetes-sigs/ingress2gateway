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

	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestEmitter_EmitCreatesAgentgatewayPolicy(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns", Name: "route"}
	bufferQty := resource.MustParse("5Mi")

	ir := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{{}},
					},
				},
				BodySizeByRuleIdx: map[int]*emitterir.BodySize{
					0: {BufferSize: &bufferQty},
				},
			},
		},
	}

	e := NewEmitter(nil)
	gatewayResources, errs := e.Emit(ir)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(gatewayResources.GatewayExtensions) != 1 {
		t.Fatalf("expected 1 gateway extension, got %d", len(gatewayResources.GatewayExtensions))
	}

	policy := agentgatewayv1alpha1.AgentgatewayPolicy{}
	if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(gatewayResources.GatewayExtensions[0].Object, &policy); err != nil {
		t.Fatalf("failed to convert extension: %v", err)
	}

	if policy.Spec.Frontend == nil || policy.Spec.Frontend.HTTP == nil {
		t.Fatalf("expected frontend http to be populated, got %+v", policy.Spec.Frontend)
	}

	if policy.Spec.Frontend.HTTP.MaxBufferSize == nil {
		t.Fatalf("expected max buffer size to be set")
	}

	expected := int32(bufferQty.Value())
	if *policy.Spec.Frontend.HTTP.MaxBufferSize != expected {
		t.Fatalf("expected max buffer size %d, got %d", expected, *policy.Spec.Frontend.HTTP.MaxBufferSize)
	}
}

func TestEmitter_EmitPrefersMaxSize(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns", Name: "route"}
	bufferQty := resource.MustParse("1Mi")
	maxQty := resource.MustParse("10Mi")

	ir := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{{}},
					},
				},
				BodySizeByRuleIdx: map[int]*emitterir.BodySize{
					0: {BufferSize: &bufferQty, MaxSize: &maxQty},
				},
			},
		},
	}

	e := NewEmitter(nil)
	gatewayResources, errs := e.Emit(ir)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(gatewayResources.GatewayExtensions) != 1 {
		t.Fatalf("expected 1 gateway extension, got %d", len(gatewayResources.GatewayExtensions))
	}

	policy := agentgatewayv1alpha1.AgentgatewayPolicy{}
	if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(gatewayResources.GatewayExtensions[0].Object, &policy); err != nil {
		t.Fatalf("failed to convert extension: %v", err)
	}

	if policy.Spec.Frontend == nil || policy.Spec.Frontend.HTTP == nil || policy.Spec.Frontend.HTTP.MaxBufferSize == nil {
		t.Fatalf("expected max buffer size to be set")
	}

	expected := int32(maxQty.Value())
	if *policy.Spec.Frontend.HTTP.MaxBufferSize != expected {
		t.Fatalf("expected max buffer size %d, got %d", expected, *policy.Spec.Frontend.HTTP.MaxBufferSize)
	}
}
