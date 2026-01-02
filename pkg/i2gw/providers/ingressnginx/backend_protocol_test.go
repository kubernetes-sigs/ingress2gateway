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

package ingressnginx

import (
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBackendProtocolFeature(t *testing.T) {
	common.GRPCRouteGVK.Group = "gateway.networking.k8s.io"
	common.GRPCRouteGVK.Version = "v1"
	common.GRPCRouteGVK.Kind = "GRPCRoute"

	testCases := []struct {
		name                string
		ingresses           []networkingv1.Ingress
		expectedHTTP        map[types.NamespacedName]int // Count of HTTP routes expected
		expectedGRPC        map[types.NamespacedName]int // Count of GRPC routes expected
		expectedGVK         bool                         // Verify GVK is set correctly
		expectedProtocol    string                       // Verify backend protocol was respected
		expectedTLSPolicies map[types.NamespacedName]int // Count of BackendTLSPolicies expected
	}{
		{
			name: "No backend protocol annotation - should result in HTTPRoute",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "test-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 80,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHTTP: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-example-com"}: 1,
			},
			expectedGRPC: map[types.NamespacedName]int{},
		},
		{
			name: "backend protocol GRPC - should result in GRPCRoute",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-grpc",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "grpc.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "grpc-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 50051,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHTTP: map[types.NamespacedName]int{},
			expectedGRPC: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-grpc-grpc-example-com"}: 1,
			},
			expectedGVK: true,
		},

		{
			name: "backend protocol GRPCS - should result in GRPCRoute + BackendTLSPolicy",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-grpcs",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "GRPCS",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "grpcs.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "grpcs-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 443,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHTTP: map[types.NamespacedName]int{},
			expectedGRPC: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-grpcs-grpcs-example-com"}: 1,
			},
			expectedGVK: true,
			expectedTLSPolicies: map[types.NamespacedName]int{
				{Namespace: "default", Name: "grpcs-service-tls-policy"}: 1,
			},
		},
		{
			name: "backend protocol HTTPS - should result in HTTPRoute + BackendTLSPolicy",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-https",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "https.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "https-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 443,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHTTP: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-https-https-example-com"}: 1,
			},
			expectedGRPC: map[types.NamespacedName]int{},
			expectedGVK:  false, // HTTPRoute
			expectedTLSPolicies: map[types.NamespacedName]int{
				{Namespace: "default", Name: "https-service-tls-policy"}: 1,
			},
		},
		{
			name: "backend protocol FCGI - should result in HTTPRoute (and warning logged)",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-fcgi",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "FCGI",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "fcgi.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "fcgi-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 9000,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHTTP: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-fcgi-fcgi-example-com"}: 1,
			},
			expectedGRPC: map[types.NamespacedName]int{},
		},
		{
			name: "backend protocol HTTP - should result in HTTPRoute",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-http",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "HTTP",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "http.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "http-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 80,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHTTP: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-http-http-example-com"}: 1,
			},
			expectedGRPC: map[types.NamespacedName]int{},
		},
		{
			name: "backend protocol AUTO_HTTP - should result in HTTPRoute",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-auto-http",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "AUTO_HTTP",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "auto.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "auto-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 80,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHTTP: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-auto-http-auto-example-com"}: 1,
			},
			expectedGRPC: map[types.NamespacedName]int{},
		},
		{
			name: "mixed protocol (HTTP and GRPC on same host) - should result in split routes",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-mixed-http",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "HTTP",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "mixed.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/api",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "http-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 80,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-mixed-grpc",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "mixed.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/grpc",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "grpc-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 9000,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			// Both ingresses will effectively map to the same RouteNamebase (test-ingress-mixed-http-mixed-example-com)
			// because common.ToIR uses the first ingress name matching the host key.
			// Wait, common.ToIR uses "first ingress" to determine name.
			// If we pass a list, order matters. But they share the key.
			// Let's rely on checking existence of Routes for that host key.

			expectedHTTP: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-mixed-http-mixed-example-com"}: 1,
			},
			expectedGRPC: map[types.NamespacedName]int{
				{Namespace: "default", Name: "test-ingress-mixed-grpc-mixed-example-com"}: 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			servicePorts := map[types.NamespacedName]map[string]int32{}

			// Simulate converter.go logic:
			// Filter Ingresses
			var httpIngresses []networkingv1.Ingress
			var grpcIngresses []networkingv1.Ingress

			for _, ing := range tc.ingresses {
				if val, ok := ing.Annotations["nginx.ingress.kubernetes.io/backend-protocol"]; ok && (val == "GRPC" || val == "GRPCS") {
					grpcIngresses = append(grpcIngresses, ing)
				} else {
					httpIngresses = append(httpIngresses, ing)
				}
			}

			// Use common.ToIR to convert ingresses
			ir, errs := common.ToIR(httpIngresses, grpcIngresses, servicePorts, i2gw.ProviderImplementationSpecificOptions{})
			if len(errs) > 0 {
				t.Fatalf("common.ToIR returned errors: %v", errs)
			}

			// createBackendTLSPolicies for ALL
			tlsErrs := createBackendTLSPolicies(tc.ingresses, servicePorts, &ir)
			if len(tlsErrs) > 0 {
				t.Fatalf("createBackendTLSPolicies returned errors: %v", tlsErrs)
			}

			// Verify HTTPRoutes
			if len(ir.HTTPRoutes) != len(tc.expectedHTTP) {
				t.Errorf("Expected %d HTTPRoutes, got %d", len(tc.expectedHTTP), len(ir.HTTPRoutes))
			}
			for key := range tc.expectedHTTP {
				if _, ok := ir.HTTPRoutes[key]; !ok {
					t.Errorf("Expected HTTPRoute %v not found", key)
				}
			}

			// Verify GRPCRoutes
			if len(ir.GRPCRoutes) != len(tc.expectedGRPC) {
				t.Errorf("Expected %d GRPCRoutes, got %d", len(tc.expectedGRPC), len(ir.GRPCRoutes))
			}
			for key := range tc.expectedGRPC {
				route, ok := ir.GRPCRoutes[key]
				if !ok {
					t.Errorf("Expected GRPCRoute %v not found", key)
				}
				if tc.expectedGVK {
					if route.GroupVersionKind() != common.GRPCRouteGVK {
						t.Errorf("Expected GVK %v, got %v", common.GRPCRouteGVK, route.GroupVersionKind())
					}
				}
				if len(route.Spec.Rules) == 0 {
					t.Errorf("Expected rules in GRPCRoute, got 0")
				} else {
					// Check that there are no matches (catch-all) as per implementation
					if len(route.Spec.Rules[0].Matches) != 0 {
						t.Errorf("Expected 0 matches (catch-all) in GRPCRoute rule, got %d", len(route.Spec.Rules[0].Matches))
					}
				}
			}

			// Verify BackendTLSPolicies
			if len(ir.BackendTLSPolicies) != len(tc.expectedTLSPolicies) {
				t.Errorf("Expected %d BackendTLSPolicies, got %d", len(tc.expectedTLSPolicies), len(ir.BackendTLSPolicies))
			}
			for key := range tc.expectedTLSPolicies {
				if _, ok := ir.BackendTLSPolicies[key]; !ok {
					t.Errorf("Expected BackendTLSPolicy %v not found", key)
				}
			}
		})
	}
}
