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

package ingressnginx

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
)

func TestHasBackendTLSAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    false,
		},
		{
			name:        "no annotations",
			annotations: map[string]string{},
			expected:    false,
		},
		{
			name: "unrelated annotations",
			annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
			expected: false,
		},
		{
			name: "proxy-ssl-secret annotation",
			annotations: map[string]string{
				ProxySSLSecretAnnotation: "default/my-cert",
			},
			expected: true,
		},
		{
			name: "proxy-ssl-name annotation",
			annotations: map[string]string{
				ProxySSLNameAnnotation: "backend.example.com",
			},
			expected: true,
		},
		{
			name: "proxy-ssl-verify annotation",
			annotations: map[string]string{
				ProxySSLVerifyAnnotation: "on",
			},
			expected: true,
		},
		{
			name: "multiple backend TLS annotations",
			annotations: map[string]string{
				ProxySSLSecretAnnotation:    "default/my-cert",
				ProxySSLNameAnnotation:      "backend.example.com",
				ProxySSLVerifyAnnotation:    "on",
				ProxySSLProtocolsAnnotation: "TLSv1.2 TLSv1.3",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			result := hasBackendTLSAnnotations(ingress)
			if result != tt.expected {
				t.Errorf("hasBackendTLSAnnotations() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestParseSecretReference(t *testing.T) {
	tests := []struct {
		name              string
		ref               string
		defaultNamespace  string
		expectedNamespace string
		expectedName      string
		expectError       bool
	}{
		{
			name:              "namespace/name format",
			ref:               "my-namespace/my-secret",
			defaultNamespace:  "default",
			expectedNamespace: "my-namespace",
			expectedName:      "my-secret",
			expectError:       false,
		},
		{
			name:              "name only format",
			ref:               "my-secret",
			defaultNamespace:  "default",
			expectedNamespace: "default",
			expectedName:      "my-secret",
			expectError:       false,
		},
		{
			name:              "with spaces trimmed",
			ref:               "  my-namespace / my-secret  ",
			defaultNamespace:  "default",
			expectedNamespace: "my-namespace",
			expectedName:      "my-secret",
			expectError:       false,
		},
		{
			name:             "empty reference",
			ref:              "",
			defaultNamespace: "default",
			expectError:      true,
		},
		{
			name:             "empty reference with spaces",
			ref:              "   ",
			defaultNamespace: "default",
			expectError:      true,
		},
		{
			name:             "empty namespace",
			ref:              "/my-secret",
			defaultNamespace: "default",
			expectError:      true,
		},
		{
			name:             "empty name",
			ref:              "my-namespace/",
			defaultNamespace: "default",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace, name, err := parseSecretReference(tt.ref, tt.defaultNamespace)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if namespace != tt.expectedNamespace {
				t.Errorf("namespace = %q, expected %q", namespace, tt.expectedNamespace)
			}
			if name != tt.expectedName {
				t.Errorf("name = %q, expected %q", name, tt.expectedName)
			}
		})
	}
}

func TestParseBackendTLSConfig(t *testing.T) {
	t.Run("valid config with all annotations", func(t *testing.T) {
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Annotations: map[string]string{
					ProxySSLSecretAnnotation:      "other-ns/my-cert",
					ProxySSLCiphersAnnotation:     "HIGH:!aNULL:!MD5",
					ProxySSLNameAnnotation:        "backend.example.com",
					ProxySSLProtocolsAnnotation:   "TLSv1.2 TLSv1.3",
					ProxySSLVerifyAnnotation:      "on",
					ProxySSLVerifyDepthAnnotation: "2",
					ProxySSLServerNameAnnotation:  "on",
				},
			},
		}

		config, errs := parseBackendTLSConfig(ingress)

		if len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}

		if config.Secret != "other-ns/my-cert" {
			t.Errorf("Secret = %q, expected %q", config.Secret, "other-ns/my-cert")
		}
		if config.SecretNamespace != "other-ns" {
			t.Errorf("SecretNamespace = %q, expected %q", config.SecretNamespace, "other-ns")
		}
		if config.SecretName != "my-cert" {
			t.Errorf("SecretName = %q, expected %q", config.SecretName, "my-cert")
		}
		if config.Ciphers != "HIGH:!aNULL:!MD5" {
			t.Errorf("Ciphers = %q, expected %q", config.Ciphers, "HIGH:!aNULL:!MD5")
		}
		if config.Name != "backend.example.com" {
			t.Errorf("Name = %q, expected %q", config.Name, "backend.example.com")
		}
		if config.Protocols != "TLSv1.2 TLSv1.3" {
			t.Errorf("Protocols = %q, expected %q", config.Protocols, "TLSv1.2 TLSv1.3")
		}
		if config.Verify != "on" {
			t.Errorf("Verify = %q, expected %q", config.Verify, "on")
		}
		if config.VerifyDepth != "2" {
			t.Errorf("VerifyDepth = %q, expected %q", config.VerifyDepth, "2")
		}
		if config.ServerName != "on" {
			t.Errorf("ServerName = %q, expected %q", config.ServerName, "on")
		}
	})

	t.Run("secret without namespace uses ingress namespace", func(t *testing.T) {
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "my-namespace",
				Annotations: map[string]string{
					ProxySSLSecretAnnotation: "my-cert",
				},
			},
		}

		config, errs := parseBackendTLSConfig(ingress)

		if len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}

		if config.SecretNamespace != "my-namespace" {
			t.Errorf("SecretNamespace = %q, expected %q", config.SecretNamespace, "my-namespace")
		}
		if config.SecretName != "my-cert" {
			t.Errorf("SecretName = %q, expected %q", config.SecretName, "my-cert")
		}
	})

	t.Run("invalid secret reference returns error", func(t *testing.T) {
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-ingress",
				Annotations: map[string]string{
					ProxySSLSecretAnnotation: "/invalid",
				},
			},
		}

		_, errs := parseBackendTLSConfig(ingress)

		if len(errs) == 0 {
			t.Error("expected error for invalid secret reference")
		}
	})
}

func TestCollectBackendServices(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "default-svc",
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/api",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-svc",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
								},
								{
									Path:     "/web",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "web-svc",
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

	services := collectBackendServices(ingress)

	expectedServices := []string{"default-svc", "api-svc", "web-svc"}
	if len(services) != len(expectedServices) {
		t.Errorf("Expected %d services, got %d", len(expectedServices), len(services))
	}

	for _, svc := range expectedServices {
		if _, ok := services[svc]; !ok {
			t.Errorf("Expected service %q not found", svc)
		}
	}
}

func TestCreateBackendTLSPolicy(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: "default",
		},
	}

	tests := []struct {
		name            string
		serviceName     string
		config          backendTLSConfig
		expectedName    string
		expectedHost    gatewayv1.PreciseHostname
	}{
		{
			name:            "policy without explicit hostname uses service name",
			serviceName:     "my-service",
			config:          backendTLSConfig{},
			expectedName:    "my-ingress-my-service-backend-tls",
			expectedHost:    "my-service",
		},
		{
			name:         "policy with explicit hostname",
			serviceName:  "api-service",
			config:       backendTLSConfig{Name: "api.backend.local"},
			expectedName: "my-ingress-api-service-backend-tls",
			expectedHost: "api.backend.local",
		},
		{
			name:        "policy with secret reference",
			serviceName: "secure-service",
			config: backendTLSConfig{
				SecretNamespace: "default",
				SecretName:      "client-cert",
				Name:            "secure.backend.local",
			},
			expectedName: "my-ingress-secure-service-backend-tls",
			expectedHost: "secure.backend.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := createBackendTLSPolicy(ingress, tt.serviceName, tt.config)

			if policy.Name != tt.expectedName {
				t.Errorf("Policy name = %q, expected %q", policy.Name, tt.expectedName)
			}

			if policy.Namespace != ingress.Namespace {
				t.Errorf("Policy namespace = %q, expected %q", policy.Namespace, ingress.Namespace)
			}

			if policy.Kind != "BackendTLSPolicy" {
				t.Errorf("Policy kind = %q, expected %q", policy.Kind, "BackendTLSPolicy")
			}

			if len(policy.Spec.TargetRefs) != 1 {
				t.Fatalf("Expected 1 target ref, got %d", len(policy.Spec.TargetRefs))
			}

			targetRef := policy.Spec.TargetRefs[0]
			if string(targetRef.Name) != tt.serviceName {
				t.Errorf("TargetRef name = %q, expected %q", targetRef.Name, tt.serviceName)
			}
			if targetRef.Kind != "Service" {
				t.Errorf("TargetRef kind = %q, expected %q", targetRef.Kind, "Service")
			}

			if policy.Spec.Validation.Hostname != tt.expectedHost {
				t.Errorf("Hostname = %q, expected %q", policy.Spec.Validation.Hostname, tt.expectedHost)
			}
		})
	}
}

func TestBackendTLSFeature(t *testing.T) {
	pathType := networkingv1.PathTypePrefix

	tests := []struct {
		name              string
		ingresses         []networkingv1.Ingress
		expectedPolicies  int
		expectedPolicyKey types.NamespacedName
	}{
		{
			name: "ingress without backend TLS annotations",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "basic-ingress",
						Namespace:   "default",
						Annotations: map[string]string{},
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
												PathType: &pathType,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "my-service",
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
				},
			},
			expectedPolicies: 0,
		},
		{
			name: "ingress with proxy-ssl-name annotation",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tls-ingress",
						Namespace: "default",
						Annotations: map[string]string{
							ProxySSLNameAnnotation: "backend.example.com",
						},
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
												PathType: &pathType,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "backend-service",
														Port: networkingv1.ServiceBackendPort{Number: 443},
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
			expectedPolicies:  1,
			expectedPolicyKey: types.NamespacedName{Namespace: "default", Name: "tls-ingress-backend-service-backend-tls"},
		},
		{
			name: "ingress with multiple services and backend TLS",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-svc-ingress",
						Namespace: "production",
						Annotations: map[string]string{
							ProxySSLVerifyAnnotation: "on",
							ProxySSLNameAnnotation:   "internal.cluster.local",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "api.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/v1",
												PathType: &pathType,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "api-v1",
														Port: networkingv1.ServiceBackendPort{Number: 443},
													},
												},
											},
											{
												Path:     "/v2",
												PathType: &pathType,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "api-v2",
														Port: networkingv1.ServiceBackendPort{Number: 443},
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
			expectedPolicies: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := &providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			errs := backendTLSFeature(tt.ingresses, nil, ir)
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			if ir.BackendTLSPolicies == nil && tt.expectedPolicies > 0 {
				t.Fatalf("Expected BackendTLSPolicies map to be initialized")
			}

			if tt.expectedPolicies > 0 {
				if len(ir.BackendTLSPolicies) != tt.expectedPolicies {
					t.Errorf("Expected %d policies, got %d", tt.expectedPolicies, len(ir.BackendTLSPolicies))
				}
			}

			if tt.expectedPolicyKey.Name != "" {
				if _, ok := ir.BackendTLSPolicies[tt.expectedPolicyKey]; !ok {
					t.Errorf("Expected policy %v not found", tt.expectedPolicyKey)
				}
			}
		})
	}
}

func TestBackendTLSFeature_PolicyHostname(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	ingresses := []networkingv1.Ingress{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: "default",
				Annotations: map[string]string{
					ProxySSLNameAnnotation: "secure-backend.internal",
				},
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
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "my-service",
												Port: networkingv1.ServiceBackendPort{Number: 443},
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
	}

	ir := &providerir.ProviderIR{
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}

	errs := backendTLSFeature(ingresses, nil, ir)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	policyKey := types.NamespacedName{Namespace: "default", Name: "test-ingress-my-service-backend-tls"}
	policy, ok := ir.BackendTLSPolicies[policyKey]
	if !ok {
		t.Fatalf("Expected policy %v not found", policyKey)
	}

	if policy.Spec.Validation.Hostname != "secure-backend.internal" {
		t.Errorf("Policy hostname = %q, expected %q", policy.Spec.Validation.Hostname, "secure-backend.internal")
	}
}

func TestBackendTLSFeature_DefaultHostname(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	ingresses := []networkingv1.Ingress{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: "default",
				Annotations: map[string]string{
					ProxySSLVerifyAnnotation: "on", // Triggers backend TLS but no explicit hostname
				},
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
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "backend-svc",
												Port: networkingv1.ServiceBackendPort{Number: 443},
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
	}

	ir := &providerir.ProviderIR{
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}

	errs := backendTLSFeature(ingresses, nil, ir)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	policyKey := types.NamespacedName{Namespace: "default", Name: "test-ingress-backend-svc-backend-tls"}
	policy, ok := ir.BackendTLSPolicies[policyKey]
	if !ok {
		t.Fatalf("Expected policy %v not found", policyKey)
	}

	// Should use service name as default hostname
	if policy.Spec.Validation.Hostname != "backend-svc" {
		t.Errorf("Policy hostname = %q, expected %q (service name as default)", policy.Spec.Validation.Hostname, "backend-svc")
	}
}

func TestBackendTLSFeature_InvalidSecretReference(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	ingresses := []networkingv1.Ingress{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: "default",
				Annotations: map[string]string{
					ProxySSLSecretAnnotation: "/invalid-secret",
				},
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
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "my-service",
												Port: networkingv1.ServiceBackendPort{Number: 443},
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
	}

	ir := &providerir.ProviderIR{
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}

	errs := backendTLSFeature(ingresses, nil, ir)
	if len(errs) == 0 {
		t.Error("Expected error for invalid secret reference")
	}
}
