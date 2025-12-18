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

package annotations

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

func TestGRPCServicesRemoveHTTPRoute(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grpc-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				nginxGRPCServicesAnnotation: "grpc-service",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{
				{
					Host: "grpc.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/grpc.service/Method",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "grpc-service",
											Port: networkingv1.ServiceBackendPort{Number: 50051},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Setup IR with an existing HTTPRoute that should be removed
	routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
	routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

	ir := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			routeKey: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: ingress.Namespace,
					},
				},
			},
		},
		GRPCRoutes:         make(map[types.NamespacedName]gatewayv1.GRPCRoute),
		BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy),
	}

	// Verify HTTPRoute exists before
	if _, exists := ir.HTTPRoutes[routeKey]; !exists {
		t.Fatal("HTTPRoute should exist before calling GRPCServicesFeature")
	}

	// Execute
	errs := GRPCServicesFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Errorf("Unexpected errors: %v", errs)
		return
	}

	// Debug output
	t.Logf("HTTPRoutes after: %d", len(ir.HTTPRoutes))
	t.Logf("GRPCRoutes after: %d", len(ir.GRPCRoutes))
	t.Logf("Expected routeKey: %s", routeKey)
	for k := range ir.HTTPRoutes {
		t.Logf("HTTPRoute key: %s", k)
	}
	for k := range ir.GRPCRoutes {
		t.Logf("GRPCRoute key: %s", k)
	}

	// Verify HTTPRoute was removed
	if _, exists := ir.HTTPRoutes[routeKey]; exists {
		t.Error("HTTPRoute should be removed for gRPC services")
	}

	// Verify GRPCRoute was created
	if _, exists := ir.GRPCRoutes[routeKey]; !exists {
		t.Error("GRPCRoute should be created for gRPC services")
		return // Don't continue testing structure if route doesn't exist
	}

	// Verify GRPCRoute structure
	grpcRoute := ir.GRPCRoutes[routeKey]
	expectedRules := 1
	if len(grpcRoute.Spec.Rules) != expectedRules {
		t.Errorf("Expected GRPCRoute to have %d rules, got %d", expectedRules, len(grpcRoute.Spec.Rules))
		return
	}

	expectedBackendRefs := 1
	if len(grpcRoute.Spec.Rules[0].BackendRefs) != expectedBackendRefs {
		t.Errorf("Expected GRPCRoute to have %d backend refs, got %d", expectedBackendRefs, len(grpcRoute.Spec.Rules[0].BackendRefs))
		return
	}

	backendRef := grpcRoute.Spec.Rules[0].BackendRefs[0]
	if string(backendRef.BackendRef.BackendObjectReference.Name) != "grpc-service" {
		t.Errorf("Expected backend service 'grpc-service', got '%s'", backendRef.BackendRef.BackendObjectReference.Name)
	}
}

func TestGRPCServicesWithMixedServices(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mixed-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				nginxGRPCServicesAnnotation: "grpc-service",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{
				{
					Host: "mixed.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/grpc.service/Method",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "grpc-service",
											Port: networkingv1.ServiceBackendPort{Number: 50051},
										},
									},
								},
								{
									Path:     "/api/v1",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "http-service",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Setup IR with existing HTTPRoute containing filters
	routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
	routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

	httpRouteRules := []gatewayv1.HTTPRouteRule{
		{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
						Value: ptr.To("/grpc.service/Method"),
					},
				},
			},
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "grpc-service",
							Port: ptr.To(gatewayv1.PortNumber(50051)),
						},
					},
				},
			},
			Filters: []gatewayv1.HTTPRouteFilter{
				{
					Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Set: []gatewayv1.HTTPHeader{
							{Name: "X-Custom-Header", Value: "test-value"},
						},
					},
				},
				{
					Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Remove: []string{"Server"},
					},
				},
			},
		},
		{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
						Value: ptr.To("/api/v1"),
					},
				},
			},
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "http-service",
							Port: ptr.To(gatewayv1.PortNumber(8080)),
						},
					},
				},
			},
			Filters: []gatewayv1.HTTPRouteFilter{
				{
					Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Add: []gatewayv1.HTTPHeader{
							{Name: "X-API-Version", Value: "v1"},
						},
					},
				},
			},
		},
	}

	ir := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			routeKey: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: ingress.Namespace,
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: httpRouteRules,
					},
				},
			},
		},
		GRPCRoutes: make(map[types.NamespacedName]gatewayv1.GRPCRoute),
	}

	// Execute
	errs := GRPCServicesFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Errorf("Unexpected errors: %v", errs)
		return
	}

	// Verify HTTPRoute still exists (but modified)
	httpRouteContext, httpExists := ir.HTTPRoutes[routeKey]
	if !httpExists {
		t.Error("HTTPRoute should still exist for mixed services")
		return
	}

	// Verify HTTPRoute only has non-gRPC rules
	if len(httpRouteContext.HTTPRoute.Spec.Rules) != 1 {
		t.Errorf("Expected 1 remaining HTTPRoute rule, got %d", len(httpRouteContext.HTTPRoute.Spec.Rules))
		return
	}

	remainingRule := httpRouteContext.HTTPRoute.Spec.Rules[0]
	if len(remainingRule.BackendRefs) != 1 {
		t.Errorf("Expected 1 backend ref in remaining rule, got %d", len(remainingRule.BackendRefs))
		return
	}

	if string(remainingRule.BackendRefs[0].BackendRef.BackendObjectReference.Name) != "http-service" {
		t.Errorf("Expected remaining backend to be 'http-service', got '%s'",
			remainingRule.BackendRefs[0].BackendRef.BackendObjectReference.Name)
	}

	// Verify GRPCRoute was created
	grpcRoute, grpcExists := ir.GRPCRoutes[routeKey]
	if !grpcExists {
		t.Error("GRPCRoute should be created for gRPC services")
		return
	}

	// Verify GRPCRoute structure
	if len(grpcRoute.Spec.Rules) != 1 {
		t.Errorf("Expected 1 GRPCRoute rule, got %d", len(grpcRoute.Spec.Rules))
		return
	}

	grpcRule := grpcRoute.Spec.Rules[0]
	if len(grpcRule.BackendRefs) != 1 {
		t.Errorf("Expected 1 gRPC backend ref, got %d", len(grpcRule.BackendRefs))
		return
	}

	if string(grpcRule.BackendRefs[0].BackendRef.BackendObjectReference.Name) != "grpc-service" {
		t.Errorf("Expected gRPC backend to be 'grpc-service', got '%s'",
			grpcRule.BackendRefs[0].BackendRef.BackendObjectReference.Name)
	}

	// Verify filters were copied to GRPCRoute
	if len(grpcRule.Filters) != 2 {
		t.Errorf("Expected 2 filters in GRPCRoute rule, got %d", len(grpcRule.Filters))
		return
	}

	// Check RequestHeaderModifier filter
	var hasRequestFilter, hasResponseFilter bool
	for _, filter := range grpcRule.Filters {
		if filter.Type == gatewayv1.GRPCRouteFilterRequestHeaderModifier {
			hasRequestFilter = true
			if filter.RequestHeaderModifier == nil {
				t.Error("RequestHeaderModifier should not be nil")
			} else if len(filter.RequestHeaderModifier.Set) != 1 ||
				string(filter.RequestHeaderModifier.Set[0].Name) != "X-Custom-Header" ||
				filter.RequestHeaderModifier.Set[0].Value != "test-value" {
				t.Error("RequestHeaderModifier not correctly copied")
			}
		}
		if filter.Type == gatewayv1.GRPCRouteFilterResponseHeaderModifier {
			hasResponseFilter = true
			if filter.ResponseHeaderModifier == nil {
				t.Error("ResponseHeaderModifier should not be nil")
			} else if len(filter.ResponseHeaderModifier.Remove) != 1 ||
				filter.ResponseHeaderModifier.Remove[0] != "Server" {
				t.Error("ResponseHeaderModifier not correctly copied")
			}
		}
	}

	if !hasRequestFilter {
		t.Error("GRPCRoute should have RequestHeaderModifier filter")
	}
	if !hasResponseFilter {
		t.Error("GRPCRoute should have ResponseHeaderModifier filter")
	}
}
