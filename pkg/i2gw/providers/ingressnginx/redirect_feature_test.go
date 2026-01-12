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

func TestCreateRedirectFilter(t *testing.T) {
	testCases := []struct {
		name           string
		url            string
		statusCode     int
		expectedScheme *string
		expectedHost   *gatewayv1.PreciseHostname
		expectedPort   *gatewayv1.PortNumber
		expectedPath   *string
		expectError    bool
	}{
		{
			name:           "full URL with path",
			url:            "https://example.com/new-path",
			statusCode:     301,
			expectedScheme: ptr.To("https"),
			expectedHost:   ptr.To(gatewayv1.PreciseHostname("example.com")),
			expectedPath:   ptr.To("/new-path"),
			expectError:    false,
		},
		{
			name:           "URL with port",
			url:            "https://example.com:8443",
			statusCode:     302,
			expectedScheme: ptr.To("https"),
			expectedHost:   ptr.To(gatewayv1.PreciseHostname("example.com")),
			expectedPort:   ptr.To(gatewayv1.PortNumber(8443)),
			expectError:    false,
		},
		{
			name:           "simple URL",
			url:            "https://newsite.com",
			statusCode:     301,
			expectedScheme: ptr.To("https"),
			expectedHost:   ptr.To(gatewayv1.PreciseHostname("newsite.com")),
			expectError:    false,
		},
		{
			name:           "URL with only path",
			url:            "/new-location",
			statusCode:     302,
			expectedPath:   ptr.To("/new-location"),
			expectError:    false,
		},
		{
			name:        "invalid port - out of range",
			url:         "https://example.com:99999",
			statusCode:  301,
			expectError: true,
		},
		{
			name:           "URL with query string ignored",
			url:            "https://example.com/path?foo=bar",
			statusCode:     301,
			expectedScheme: ptr.To("https"),
			expectedHost:   ptr.To(gatewayv1.PreciseHostname("example.com")),
			expectedPath:   ptr.To("/path"),
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter, err := createRedirectFilter(tc.url, tc.statusCode)
			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if filter.Type != gatewayv1.HTTPRouteFilterRequestRedirect {
				t.Errorf("Expected filter type RequestRedirect, got %s", filter.Type)
			}

			redirect := filter.RequestRedirect
			if redirect == nil {
				t.Fatalf("Expected RequestRedirect to be set")
			}

			if *redirect.StatusCode != tc.statusCode {
				t.Errorf("Expected status code %d, got %d", tc.statusCode, *redirect.StatusCode)
			}

			if tc.expectedScheme != nil {
				if redirect.Scheme == nil || *redirect.Scheme != *tc.expectedScheme {
					t.Errorf("Expected scheme %s, got %v", *tc.expectedScheme, redirect.Scheme)
				}
			}

			if tc.expectedHost != nil {
				if redirect.Hostname == nil || *redirect.Hostname != *tc.expectedHost {
					t.Errorf("Expected hostname %s, got %v", *tc.expectedHost, redirect.Hostname)
				}
			}

			if tc.expectedPort != nil {
				if redirect.Port == nil || *redirect.Port != *tc.expectedPort {
					t.Errorf("Expected port %d, got %v", *tc.expectedPort, redirect.Port)
				}
			}

			if tc.expectedPath != nil {
				if redirect.Path == nil || redirect.Path.ReplaceFullPath == nil || *redirect.Path.ReplaceFullPath != *tc.expectedPath {
					var actual string
					if redirect.Path != nil && redirect.Path.ReplaceFullPath != nil {
						actual = *redirect.Path.ReplaceFullPath
					}
					t.Errorf("Expected path %s, got %s", *tc.expectedPath, actual)
				}
			}
		})
	}
}

func TestRedirectFeature(t *testing.T) {
	testCases := []struct {
		name                 string
		ingress              networkingv1.Ingress
		expectedFilterCount  int
		expectedStatusCode   int
		expectedScheme       *string
		expectedHostname     *gatewayv1.PreciseHostname
		expectError          bool
	}{
		{
			name: "permanent-redirect",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-permanent",
					Namespace: "default",
					Annotations: map[string]string{
						PermanentRedirectAnnotation: "https://newsite.com",
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
			expectedFilterCount: 1,
			expectedStatusCode:  301,
			expectedScheme:      ptr.To("https"),
			expectedHostname:    ptr.To(gatewayv1.PreciseHostname("newsite.com")),
			expectError:         false,
		},
		{
			name: "permanent-redirect with custom code",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-permanent-code",
					Namespace: "default",
					Annotations: map[string]string{
						PermanentRedirectAnnotation:     "https://newsite.com",
						PermanentRedirectCodeAnnotation: "308",
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
			expectedFilterCount: 1,
			expectedStatusCode:  308,
			expectedScheme:      ptr.To("https"),
			expectedHostname:    ptr.To(gatewayv1.PreciseHostname("newsite.com")),
			expectError:         false,
		},
		{
			name: "temporal-redirect",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-temporal",
					Namespace: "default",
					Annotations: map[string]string{
						TemporalRedirectAnnotation: "https://maintenance.example.com",
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
			expectedFilterCount: 1,
			expectedStatusCode:  302,
			expectedScheme:      ptr.To("https"),
			expectedHostname:    ptr.To(gatewayv1.PreciseHostname("maintenance.example.com")),
			expectError:         false,
		},
		{
			name: "no redirect annotations",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-redirect",
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
			expectedFilterCount: 0,
			expectError:         false,
		},
		{
			name: "invalid redirect code",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-invalid-code",
					Namespace: "default",
					Annotations: map[string]string{
						PermanentRedirectAnnotation:     "https://newsite.com",
						PermanentRedirectCodeAnnotation: "500",
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
			expectedFilterCount: 0,
			expectError:         true,
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

			errs := redirectFeature([]networkingv1.Ingress{tc.ingress}, nil, &ir)
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

			filters := rules[0].Filters
			if len(filters) != tc.expectedFilterCount {
				t.Fatalf("Expected %d filters, got %d", tc.expectedFilterCount, len(filters))
			}

			if tc.expectedFilterCount == 0 {
				return
			}

			redirect := filters[0].RequestRedirect
			if redirect == nil {
				t.Fatalf("Expected RequestRedirect to be set")
			}

			if *redirect.StatusCode != tc.expectedStatusCode {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatusCode, *redirect.StatusCode)
			}

			if tc.expectedScheme != nil {
				if redirect.Scheme == nil || *redirect.Scheme != *tc.expectedScheme {
					t.Errorf("Expected scheme %s, got %v", *tc.expectedScheme, redirect.Scheme)
				}
			}

			if tc.expectedHostname != nil {
				if redirect.Hostname == nil || *redirect.Hostname != *tc.expectedHostname {
					t.Errorf("Expected hostname %s, got %v", *tc.expectedHostname, redirect.Hostname)
				}
			}
		})
	}
}
