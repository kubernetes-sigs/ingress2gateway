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

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestParseTimeoutValue(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "seconds as integer",
			input:       "60",
			expected:    "60s",
			expectError: false,
		},
		{
			name:        "seconds with suffix",
			input:       "30s",
			expected:    "30s",
			expectError: false,
		},
		{
			name:        "minutes with suffix",
			input:       "5m",
			expected:    "5m",
			expectError: false,
		},
		{
			name:        "hours with suffix",
			input:       "1h",
			expected:    "1h",
			expectError: false,
		},
		{
			name:        "milliseconds with suffix",
			input:       "500ms",
			expected:    "500ms",
			expectError: false,
		},
		{
			name:        "combined duration",
			input:       "1h30m",
			expected:    "1h30m",
			expectError: false,
		},
		{
			name:        "combined duration with seconds",
			input:       "5m30s",
			expected:    "5m30s",
			expectError: false,
		},
		{
			name:        "zero seconds",
			input:       "0",
			expected:    "0s",
			expectError: false,
		},
		{
			name:        "zero with suffix",
			input:       "0s",
			expected:    "0s",
			expectError: false,
		},
		{
			name:        "whitespace trimmed",
			input:       "  60  ",
			expected:    "60s",
			expectError: false,
		},
		{
			name:        "negative value",
			input:       "-1",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty value",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid value",
			input:       "abc",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid format with suffix",
			input:       "abc123s",
			expected:    "",
			expectError: true,
		},
		{
			name:        "negative with suffix",
			input:       "-5s",
			expected:    "",
			expectError: true,
		},
		{
			name:        "decimal value",
			input:       "1.5s",
			expected:    "",
			expectError: true,
		},
		{
			name:        "too many digits",
			input:       "999999s",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseTimeoutValue(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if string(*result) != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, string(*result))
			}
		})
	}
}

func TestTimeoutFeature(t *testing.T) {
	testCases := []struct {
		name                   string
		ingress                networkingv1.Ingress
		expectedRequest        *gatewayv1.Duration
		expectedBackendRequest *gatewayv1.Duration
		expectError            bool
	}{
		{
			name: "proxy-read-timeout sets Request timeout",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-read-timeout",
					Namespace: "default",
					Annotations: map[string]string{
						ProxyReadTimeoutAnnotation: "60",
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
											PathType: ptr.To(networkingv1.PathTypePrefix),
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
			expectedRequest:        ptr.To(gatewayv1.Duration("60s")),
			expectedBackendRequest: nil,
			expectError:            false,
		},
		{
			name: "proxy-connect-timeout sets BackendRequest timeout",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-connect-timeout",
					Namespace: "default",
					Annotations: map[string]string{
						ProxyConnectTimeoutAnnotation: "30",
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
											PathType: ptr.To(networkingv1.PathTypePrefix),
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
			expectedRequest:        nil,
			expectedBackendRequest: ptr.To(gatewayv1.Duration("30s")),
			expectError:            false,
		},
		{
			name: "both timeouts set",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-both-timeouts",
					Namespace: "default",
					Annotations: map[string]string{
						ProxyReadTimeoutAnnotation:    "120",
						ProxyConnectTimeoutAnnotation: "10",
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
											PathType: ptr.To(networkingv1.PathTypePrefix),
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
			expectedRequest:        ptr.To(gatewayv1.Duration("120s")),
			expectedBackendRequest: ptr.To(gatewayv1.Duration("10s")),
			expectError:            false,
		},
		{
			name: "timeout with duration suffix",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-duration-suffix",
					Namespace: "default",
					Annotations: map[string]string{
						ProxyReadTimeoutAnnotation: "5m",
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
											PathType: ptr.To(networkingv1.PathTypePrefix),
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
			expectedRequest:        ptr.To(gatewayv1.Duration("5m")),
			expectedBackendRequest: nil,
			expectError:            false,
		},
		{
			name: "no timeout annotations",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-timeouts",
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
											PathType: ptr.To(networkingv1.PathTypePrefix),
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
			expectedRequest:        nil,
			expectedBackendRequest: nil,
			expectError:            false,
		},
		{
			name: "invalid timeout value",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-invalid-timeout",
					Namespace: "default",
					Annotations: map[string]string{
						ProxyReadTimeoutAnnotation: "invalid",
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
											PathType: ptr.To(networkingv1.PathTypePrefix),
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
			expectedRequest:        nil,
			expectedBackendRequest: nil,
			expectError:            true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			key := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: common.RouteName(tc.ingress.Name, "example.com")}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tc.ingress.Namespace,
					Name:      key.Name,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/"),
									},
								},
							},
						},
					},
				},
			}
			ir.HTTPRoutes[key] = providerir.HTTPRouteContext{
				HTTPRoute: route,
				RuleBackendSources: [][]providerir.BackendSource{
					{
						{Ingress: &tc.ingress},
					},
				},
			}

			errs := timeoutFeature([]networkingv1.Ingress{tc.ingress}, nil, &ir)
			if tc.expectError {
				if len(errs) == 0 {
					t.Fatalf("Expected errors but got none")
				}
				return
			}
			if len(errs) > 0 {
				t.Fatalf("Expected no errors, got %v", errs)
			}

			result := ir.HTTPRoutes[key]
			rules := result.HTTPRoute.Spec.Rules
			if len(rules) != 1 {
				t.Fatalf("Expected 1 rule, got %d", len(rules))
			}

			timeouts := rules[0].Timeouts

			if tc.expectedRequest == nil && tc.expectedBackendRequest == nil {
				if timeouts != nil {
					t.Fatalf("Expected no timeouts, but got %+v", timeouts)
				}
				return
			}

			if timeouts == nil {
				t.Fatalf("Expected timeouts to be set")
			}

			if tc.expectedRequest != nil {
				if timeouts.Request == nil {
					t.Fatalf("Expected Request timeout to be set")
				}
				if *timeouts.Request != *tc.expectedRequest {
					t.Errorf("Expected Request timeout %s, got %s", *tc.expectedRequest, *timeouts.Request)
				}
			}

			if tc.expectedBackendRequest != nil {
				if timeouts.BackendRequest == nil {
					t.Fatalf("Expected BackendRequest timeout to be set")
				}
				if *timeouts.BackendRequest != *tc.expectedBackendRequest {
					t.Errorf("Expected BackendRequest timeout %s, got %s", *tc.expectedBackendRequest, *timeouts.BackendRequest)
				}
			}
		})
	}
}
