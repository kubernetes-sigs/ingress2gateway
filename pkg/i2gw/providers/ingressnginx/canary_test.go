/*
Copyright 2023 The Kubernetes Authors.

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

package ingressnginx

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_parseCanaryConfig(t *testing.T) {
	testCases := []struct {
		name           string
		ingress        networkingv1.Ingress
		expectedConfig canaryConfig
		expectError    bool
		errorContains  string
	}{
		{
			name: "actually get weights",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "50",
						"nginx.ingress.kubernetes.io/canary-weight-total": "100",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      50,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "actually get weights with canary-by-weight",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-by-weight":    "50",
						"nginx.ingress.kubernetes.io/canary-weight-total": "100",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      50,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "assigns default weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "50",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      50,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "weight set to 0",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "0",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      0,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "weight set to 100",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "100",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      100,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "custom weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "50",
						"nginx.ingress.kubernetes.io/canary-weight-total": "200",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      50,
				weightTotal: 200,
			},
			expectError: false,
		},
		{
			name: "no weight annotation defaults to 0",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary": "true",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      0,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "errors on non integer weight",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "50.5",
					},
				},
			},
			expectError:   true,
			errorContains: "invalid canary-weight annotation",
		},
		{
			name: "errors on non integer weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "50.5",
					},
				},
			},
			expectError:   true,
			errorContains: "invalid canary-weight-total annotation",
		},
		{
			name: "errors on invalid weight string",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "abc",
					},
				},
			},
			expectError:   true,
			errorContains: "invalid canary-weight annotation",
		},
		{
			name: "errors on invalid weight total string",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "xyz",
					},
				},
			},
			expectError:   true,
			errorContains: "invalid canary-weight-total annotation",
		},
		{
			name: "errors on negative weight",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "-10",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight must be non-negative",
		},
		{
			name: "errors on zero weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "0",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight-total must be positive",
		},
		{
			name: "errors on negative weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "-100",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight-total must be positive",
		},
		{
			name: "errors when weight exceeds total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "150",
						"nginx.ingress.kubernetes.io/canary-weight-total": "100",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight (150) exceeds canary-weight-total (100)",
		},
		{
			name: "weight equal to total is valid",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "200",
						"nginx.ingress.kubernetes.io/canary-weight-total": "200",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      200,
				weightTotal: 200,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := parseCanaryConfig(&tc.ingress)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Fatalf("expected error containing %q, got %q", tc.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(config, tc.expectedConfig, cmp.AllowUnexported(canaryConfig{})); diff != "" {
				t.Fatalf("parseCanaryConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_canaryFeature_GRPC(t *testing.T) {
	ingress1 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grpcbin",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "grpcbin.local",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/hello.HelloService/abc",
								},
							},
						},
					},
				},
			},
		},
	}
	ingress2 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grpcbin2",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
				"nginx.ingress.kubernetes.io/canary":           "true",
				"nginx.ingress.kubernetes.io/canary-weight":    "10",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "grpcbin.local",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/hello.HelloService/abc",
								},
							},
						},
					},
				},
			},
		},
	}

	ir := &providerir.ProviderIR{
		GRPCRoutes: map[types.NamespacedName]providerir.GRPCRouteContext{
			{Namespace: "default", Name: "grpcbin-grpcbin-local"}: {
				GRPCRoute: gatewayv1.GRPCRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "grpcbin-grpcbin-local",
						Namespace: "default",
					},
					Spec: gatewayv1.GRPCRouteSpec{
						Rules: []gatewayv1.GRPCRouteRule{
							{
								BackendRefs: []gatewayv1.GRPCBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "grpcbin",
											},
										},
									},
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "grpcbin2",
											},
										},
									},
								},
							},
						},
					},
				},
				RuleBackendSources: [][]providerir.BackendSource{
					{
						{Ingress: ingress1},
						{Ingress: ingress2},
					},
				},
			},
		},
	}

	errs := canaryFeature([]networkingv1.Ingress{*ingress1, *ingress2}, nil, ir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	route := ir.GRPCRoutes[types.NamespacedName{Namespace: "default", Name: "grpcbin-grpcbin-local"}]
	backendRefs := route.GRPCRoute.Spec.Rules[0].BackendRefs

	if len(backendRefs) != 2 {
		t.Fatalf("expected 2 backend refs, got %d", len(backendRefs))
	}

	if backendRefs[0].Weight == nil {
		t.Fatalf("expected weight for non-canary backend to be set, got nil")
	}
	// Non-canary weight should be 90 (100-10)
	if *backendRefs[0].Weight != 90 {
		t.Errorf("expected weight 90 for non-canary backend, got %d", *backendRefs[0].Weight)
	}

	if backendRefs[1].Weight == nil {
		t.Fatalf("expected weight for canary backend to be set, got nil")
	}
	// Canary weight should be 10
	if *backendRefs[1].Weight != 10 {
		t.Errorf("expected weight 10 for canary backend, got %d", *backendRefs[1].Weight)
	}
}

func Test_canaryFeature_GRPC_ByWeight(t *testing.T) {
	ingress1 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grpcbin",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "grpcbin.local",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/hello.HelloService/abc",
								},
							},
						},
					},
				},
			},
		},
	}
	ingress2 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grpcbin2",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
				"nginx.ingress.kubernetes.io/canary":           "true",
				"nginx.ingress.kubernetes.io/canary-by-weight": "25",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "grpcbin.local",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/hello.HelloService/abc",
								},
							},
						},
					},
				},
			},
		},
	}

	ir := &providerir.ProviderIR{
		GRPCRoutes: map[types.NamespacedName]providerir.GRPCRouteContext{
			{Namespace: "default", Name: "grpcbin-grpcbin-local"}: {
				GRPCRoute: gatewayv1.GRPCRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "grpcbin-grpcbin-local",
						Namespace: "default",
					},
					Spec: gatewayv1.GRPCRouteSpec{
						Rules: []gatewayv1.GRPCRouteRule{
							{
								BackendRefs: []gatewayv1.GRPCBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "grpcbin",
											},
										},
									},
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "grpcbin2",
											},
										},
									},
								},
							},
						},
					},
				},
				RuleBackendSources: [][]providerir.BackendSource{
					{
						{Ingress: ingress1},
						{Ingress: ingress2},
					},
				},
			},
		},
	}

	errs := canaryFeature([]networkingv1.Ingress{*ingress1, *ingress2}, nil, ir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	route := ir.GRPCRoutes[types.NamespacedName{Namespace: "default", Name: "grpcbin-grpcbin-local"}]
	backendRefs := route.GRPCRoute.Spec.Rules[0].BackendRefs

	if len(backendRefs) != 2 {
		t.Fatalf("expected 2 backend refs, got %d", len(backendRefs))
	}

	if backendRefs[0].Weight == nil {
		t.Fatalf("expected weight for non-canary backend to be set, got nil")
	}
	// Non-canary weight should be 75 (100-25)
	if *backendRefs[0].Weight != 75 {
		t.Errorf("expected weight 75 for non-canary backend, got %d", *backendRefs[0].Weight)
	}

	if backendRefs[1].Weight == nil {
		t.Fatalf("expected weight for canary backend to be set, got nil")
	}
	// Canary weight should be 25
	if *backendRefs[1].Weight != 25 {
		t.Errorf("expected weight 25 for canary backend, got %d", *backendRefs[1].Weight)
	}
}
