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
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
)

func TestSSLServicesAnnotation(t *testing.T) {
	tests := []struct {
		name             string
		annotation       string
		expectedPolicies int
		expectedServices []string
	}{
		{
			name:             "single service",
			annotation:       "secure-api",
			expectedPolicies: 1,
			expectedServices: []string{"secure-api"},
		},
		{
			name:             "multiple services",
			annotation:       "secure-api,auth-service",
			expectedPolicies: 2,
			expectedServices: []string{"secure-api", "auth-service"},
		},
		{
			name:             "spaces in annotation",
			annotation:       " secure-api , auth-service , payment-api ",
			expectedPolicies: 3,
			expectedServices: []string{"secure-api", "auth-service", "payment-api"},
		},
		{
			name:             "empty annotation",
			annotation:       "",
			expectedPolicies: 0,
			expectedServices: []string{},
		},
		{
			name:             "empty values in annotation",
			annotation:       "secure-api,,auth-service,",
			expectedPolicies: 2,
			expectedServices: []string{"secure-api", "auth-service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						nginxSSLServicesAnnotation: tt.annotation,
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "web-service",
													Port: networkingv1.ServiceBackendPort{Number: 80},
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

			ir := intermediate.IR{
				BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1alpha3.BackendTLSPolicy),
			}

			errs := processSSLServicesAnnotation(ingress, tt.annotation, &ir)
			if len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
				return
			}

			if len(ir.BackendTLSPolicies) != tt.expectedPolicies {
				t.Errorf("Expected %d BackendTLSPolicies, got %d", tt.expectedPolicies, len(ir.BackendTLSPolicies))
			}

			serviceNames := make(map[string]struct{})
			for _, policy := range ir.BackendTLSPolicies {
				if len(policy.Spec.TargetRefs) > 0 {
					serviceName := string(policy.Spec.TargetRefs[0].Name)
					serviceNames[serviceName] = struct{}{}

					if policy.Spec.TargetRefs[0].Kind != "Service" {
						t.Errorf("Expected TargetRef Kind 'Service', got '%s'", policy.Spec.TargetRefs[0].Kind)
					}
					if policy.Spec.TargetRefs[0].Group != "" {
						t.Errorf("Expected TargetRef Group '%s', got '%s'", "", policy.Spec.TargetRefs[0].Group)
					}

					if policy.Labels["app.kubernetes.io/managed-by"] != "ingress2gateway" {
						t.Errorf("Expected managed-by label 'ingress2gateway', got '%s'", policy.Labels["app.kubernetes.io/managed-by"])
					}
				}
			}

			// Verify all expected services are present
			for _, expectedService := range tt.expectedServices {
				if _, exists := serviceNames[expectedService]; !exists {
					t.Errorf("Expected BackendTLSPolicy for service '%s' not found", expectedService)
				}
			}
		})
	}
}

func TestSSLServicesFeature(t *testing.T) {
	tests := []struct {
		name             string
		ingresses        []networkingv1.Ingress
		expectedPolicies int
	}{
		{
			name: "multiple ingresses with SSL services",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-1",
						Namespace: "default",
						Annotations: map[string]string{
							nginxSSLServicesAnnotation: "api-service",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-2",
						Namespace: "default",
						Annotations: map[string]string{
							nginxSSLServicesAnnotation: "auth-service,payment-service",
						},
					},
				},
			},
			expectedPolicies: 3,
		},
		{
			name: "ingress without SSL services annotation",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress",
						Namespace: "default",
						Annotations: map[string]string{
							"other-annotation": "value",
						},
					},
				},
			},
			expectedPolicies: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := intermediate.IR{
				BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1alpha3.BackendTLSPolicy),
			}

			errs := SSLServicesFeature(tt.ingresses, nil, &ir)
			if len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
				return
			}

			if len(ir.BackendTLSPolicies) != tt.expectedPolicies {
				t.Errorf("Expected %d BackendTLSPolicies, got %d", tt.expectedPolicies, len(ir.BackendTLSPolicies))
			}
		})
	}
}
