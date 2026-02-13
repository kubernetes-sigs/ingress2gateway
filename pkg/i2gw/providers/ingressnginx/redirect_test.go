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

func Test_redirectFeature(t *testing.T) {
	tests := []struct {
		name              string
		ingress           networkingv1.Ingress
		initialHTTPRoute  *gatewayv1.HTTPRoute
		expectedHTTPRoute *gatewayv1.HTTPRoute
		expectError       bool
	}{
		{
			name: "permanent-redirect annotation",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						PermanentRedirectAnnotation: "https://example.com/new-path",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "foo.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "foo",
													Port: networkingv1.ServiceBackendPort{
														Number: 3000,
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
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-foo-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"foo.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "foo", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-foo-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"foo.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterRequestRedirect,
									RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
										Scheme:     ptr.To("https"),
										Hostname:   ptr.To(gatewayv1.PreciseHostname("example.com")),
										Path:       &gatewayv1.HTTPPathModifier{Type: gatewayv1.FullPathHTTPPathModifier, ReplaceFullPath: ptr.To("/new-path")},
										StatusCode: ptr.To(301),
									},
								},
							},
							BackendRefs: nil,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "temporal-redirect annotation",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						TemporalRedirectAnnotation: "https://example.com/temporary",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "bar.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "bar",
													Port: networkingv1.ServiceBackendPort{
														Number: 8080,
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
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-bar-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"bar.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "bar", Port: ptr.To(gatewayv1.PortNumber(8080))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-bar-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"bar.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterRequestRedirect,
									RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
										Scheme:     ptr.To("https"),
										Hostname:   ptr.To(gatewayv1.PreciseHostname("example.com")),
										Path:       &gatewayv1.HTTPPathModifier{Type: gatewayv1.FullPathHTTPPathModifier, ReplaceFullPath: ptr.To("/temporary")},
										StatusCode: ptr.To(302),
									},
								},
							},
							BackendRefs: nil,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "both annotations present should choose temporal",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						PermanentRedirectAnnotation: "https://example.com/permanent",
						TemporalRedirectAnnotation:  "https://example.com/temporal",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "conflict.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "conflict",
													Port: networkingv1.ServiceBackendPort{
														Number: 8080,
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
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-conflict-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"conflict.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "conflict", Port: ptr.To(gatewayv1.PortNumber(8080))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-conflict-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"conflict.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterRequestRedirect,
									RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
										Scheme:     ptr.To("https"),
										Hostname:   ptr.To(gatewayv1.PreciseHostname("example.com")),
										Path:       &gatewayv1.HTTPPathModifier{Type: gatewayv1.FullPathHTTPPathModifier, ReplaceFullPath: ptr.To("/temporal")},
										StatusCode: ptr.To(302),
									},
								},
							},
							BackendRefs: nil,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "no redirect annotations",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "normal.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "normal",
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
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-normal-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"normal.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "normal", Port: ptr.To(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-normal-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"normal.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "normal", Port: ptr.To(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the IR with the initial HTTPRoute
			ir := providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{},
			}

				if tt.initialHTTPRoute != nil {
				routeKey := types.NamespacedName{
					Namespace: tt.initialHTTPRoute.Namespace,
					Name:      tt.initialHTTPRoute.Name,
				}
				// Initialize RuleBackendSources to match the number of rules
				ruleBackendSources := make([][]providerir.BackendSource, len(tt.initialHTTPRoute.Spec.Rules))
				for i := range ruleBackendSources {
					ruleBackendSources[i] = []providerir.BackendSource{
						{
							Ingress: &tt.ingress,
						},
					}
				}
				ir.HTTPRoutes[routeKey] = providerir.HTTPRouteContext{
					HTTPRoute:          *tt.initialHTTPRoute,
					RuleBackendSources: ruleBackendSources,
				}
			}

			// Call the feature parser
			errs := redirectFeature([]networkingv1.Ingress{tt.ingress}, nil, &ir)

			// Check error expectations
			if tt.expectError && len(errs) == 0 {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
			}

			// If we don't expect errors, verify the HTTPRoute was modified correctly
			if !tt.expectError && tt.expectedHTTPRoute != nil {
				routeKey := types.NamespacedName{
					Namespace: tt.expectedHTTPRoute.Namespace,
					Name:      tt.expectedHTTPRoute.Name,
				}
				httpRouteContext, exists := ir.HTTPRoutes[routeKey]
				if !exists {
					t.Errorf("Expected HTTPRoute %s to exist", routeKey)
					return
				}

				actualRoute := httpRouteContext.HTTPRoute

				// Verify number of rules
				if len(actualRoute.Spec.Rules) != len(tt.expectedHTTPRoute.Spec.Rules) {
					t.Errorf("Expected %d rules, got %d", len(tt.expectedHTTPRoute.Spec.Rules), len(actualRoute.Spec.Rules))
					return
				}

				// Verify redirect filter in first rule if expected
				if len(tt.expectedHTTPRoute.Spec.Rules) > 0 && len(tt.expectedHTTPRoute.Spec.Rules[0].Filters) > 0 {
					if len(actualRoute.Spec.Rules[0].Filters) == 0 {
						t.Errorf("Expected redirect filter in first rule")
						return
					}

					actualFilter := actualRoute.Spec.Rules[0].Filters[0]
					expectedFilter := tt.expectedHTTPRoute.Spec.Rules[0].Filters[0]

					if actualFilter.Type != expectedFilter.Type {
						t.Errorf("Expected filter type %v, got %v", expectedFilter.Type, actualFilter.Type)
					}

					if actualFilter.RequestRedirect == nil {
						t.Errorf("Expected RequestRedirect to be set")
						return
					}

					expected := expectedFilter.RequestRedirect
					actual := actualFilter.RequestRedirect

					if expected.StatusCode != nil {
						if actual.StatusCode == nil {
							t.Errorf("Expected status code to be set")
						} else if *actual.StatusCode != *expected.StatusCode {
							t.Errorf("Expected status code %d, got %d", *expected.StatusCode, *actual.StatusCode)
						}
					}

					if expected.Scheme != nil {
						if actual.Scheme == nil {
							t.Errorf("Expected scheme to be set")
						} else if *actual.Scheme != *expected.Scheme {
							t.Errorf("Expected scheme %s, got %s", *expected.Scheme, *actual.Scheme)
						}
					}

					if expected.Hostname != nil {
						if actual.Hostname == nil {
							t.Errorf("Expected hostname to be set")
						} else if *actual.Hostname != *expected.Hostname {
							t.Errorf("Expected hostname %s, got %s", *expected.Hostname, *actual.Hostname)
						}
					}

					if expected.Path != nil {
						if actual.Path == nil {
							t.Errorf("Expected path to be set")
						} else {
							if actual.Path.Type != expected.Path.Type {
								t.Errorf("Expected path type %v, got %v", expected.Path.Type, actual.Path.Type)
							}
							if expected.Path.ReplaceFullPath != nil {
								if actual.Path.ReplaceFullPath == nil {
									t.Errorf("Expected ReplaceFullPath to be set")
								} else if *actual.Path.ReplaceFullPath != *expected.Path.ReplaceFullPath {
									t.Errorf("Expected ReplaceFullPath %s, got %s", *expected.Path.ReplaceFullPath, *actual.Path.ReplaceFullPath)
								}
							}
						}
					}

					// Verify BackendRefs are cleared when redirect is present
					if len(actualRoute.Spec.Rules[0].BackendRefs) != 0 {
						t.Errorf("Expected BackendRefs to be cleared for redirect rule, got %d refs", len(actualRoute.Spec.Rules[0].BackendRefs))
					}
				}
			}
		})
	}
}

func Test_redirectFeature_emptyURL(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				PermanentRedirectAnnotation: "",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "empty.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "empty",
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
	}

		ir := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			{Namespace: "default", Name: common.RouteName("test-ingress", "empty.com")}: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      common.RouteName("test-ingress", "empty.com"),
						Namespace: "default",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{{}},
					},
				},
				RuleBackendSources: [][]providerir.BackendSource{
					{
						{Ingress: &ingress},
					},
				},
			},
		},
	}

	errs := redirectFeature([]networkingv1.Ingress{ingress}, nil, &ir)

	if len(errs) == 0 {
		t.Errorf("Expected error for empty redirect URL")
	}
}
