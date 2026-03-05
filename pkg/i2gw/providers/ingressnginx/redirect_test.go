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

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
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

func TestAddDefaultSSLRedirect_enabled(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	parentRefs := []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing",
			Annotations: map[string]string{
				// no SSLRedirectAnnotation -> default enabled
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: append([]gatewayv1.ParentReference(nil), parentRefs...),
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{}}},
			},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ing}, &pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route %v to be added", redirectKey)
	}

	if len(redirectCtx.Spec.ParentRefs) != 1 || redirectCtx.Spec.ParentRefs[0].Port == nil || *redirectCtx.Spec.ParentRefs[0].Port != 80 {
		t.Fatalf("expected redirect route parentRef port 80, got %#v", redirectCtx.Spec.ParentRefs)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 || origCtx.Spec.ParentRefs[0].Port == nil || *origCtx.Spec.ParentRefs[0].Port != 443 {
		t.Fatalf("expected original route parentRef port 443, got %#v", origCtx.Spec.ParentRefs)
	}

	if len(redirectCtx.Spec.Rules) != 1 || len(redirectCtx.Spec.Rules[0].Filters) != 1 {
		t.Fatalf("expected redirect route to have 1 rule with 1 filter, got %#v", redirectCtx.Spec.Rules)
	}

	f := redirectCtx.Spec.Rules[0].Filters[0]
	if f.Type != gatewayv1.HTTPRouteFilterRequestRedirect || f.RequestRedirect == nil {
		t.Fatalf("expected RequestRedirect filter, got %#v", f)
	}
	if f.RequestRedirect.Scheme == nil || *f.RequestRedirect.Scheme != "https" {
		t.Fatalf("expected scheme https, got %#v", f.RequestRedirect.Scheme)
	}
	if f.RequestRedirect.StatusCode == nil || *f.RequestRedirect.StatusCode != 308 {
		t.Fatalf("expected status code 308, got %#v", f.RequestRedirect.StatusCode)
	}
}

func TestAddDefaultSSLRedirect_disabledByAnnotation(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ing}, &pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	if _, ok := eIR.HTTPRoutes[redirectKey]; ok {
		t.Fatalf("did not expect redirect route %v to be added", redirectKey)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parentRef, got %#v", origCtx.Spec.ParentRefs)
	}
	if origCtx.Spec.ParentRefs[0].Port != nil {
		t.Fatalf("expected original route parentRef port to remain nil, got %#v", origCtx.Spec.ParentRefs[0].Port)
	}
}

func TestAddDefaultSSLRedirect_conflictingAnnotations(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	parentRefs := []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}}

	ingEnabled := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-enabled",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	ingDisabled := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-disabled",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	pathA := "/a"
	pathB := "/b"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: append([]gatewayv1.ParentReference(nil), parentRefs...),
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
			},
		},
	}

	// Two rules, each from a different ingress with conflicting ssl-redirect values.
	// Per-rule semantics: only the rule from ingEnabled should get a redirect.
	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingEnabled}},
			{{Ingress: &ingDisabled}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingEnabled, ingDisabled}, &pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route %v to be created for the enabled rule", redirectKey)
	}

	// Only one redirect rule (for /a), not two.
	if len(redirectCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 redirect rule, got %d", len(redirectCtx.Spec.Rules))
	}

	if len(redirectCtx.Spec.Rules[0].Matches) != 1 || *redirectCtx.Spec.Rules[0].Matches[0].Path.Value != "/a" {
		t.Fatalf("expected redirect rule to match /a, got %#v", redirectCtx.Spec.Rules[0].Matches)
	}

	// A passthrough route should be created on port 80 for /b.
	httpKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-http"}
	httpCtx, ok := eIR.HTTPRoutes[httpKey]
	if !ok {
		t.Fatalf("expected passthrough route %v to be created for non-redirect paths", httpKey)
	}
	if len(httpCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 passthrough rule, got %d", len(httpCtx.Spec.Rules))
	}
	if len(httpCtx.Spec.Rules[0].Matches) != 1 || *httpCtx.Spec.Rules[0].Matches[0].Path.Value != "/b" {
		t.Fatalf("expected passthrough rule to match /b, got %#v", httpCtx.Spec.Rules[0].Matches)
	}
	if len(httpCtx.Spec.ParentRefs) != 1 || httpCtx.Spec.ParentRefs[0].Port == nil || *httpCtx.Spec.ParentRefs[0].Port != 80 {
		t.Fatalf("expected passthrough route parentRef port 80, got %#v", httpCtx.Spec.ParentRefs)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 || origCtx.Spec.ParentRefs[0].Port == nil || *origCtx.Spec.ParentRefs[0].Port != 443 {
		t.Fatalf("expected original route parentRef port 443, got %#v", origCtx.Spec.ParentRefs)
	}
}

func TestAddDefaultSSLRedirect_allRulesDisabled(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ingDisabledA := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-disabled-a",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}
	ingDisabledB := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-disabled-b",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	pathA := "/a"
	pathB := "/b"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
			},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingDisabledA}},
			{{Ingress: &ingDisabledB}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingDisabledA, ingDisabledB}, &pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	if _, ok := eIR.HTTPRoutes[redirectKey]; ok {
		t.Fatalf("did not expect redirect route when all rules have ssl-redirect=false")
	}

	httpKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-http"}
	if _, ok := eIR.HTTPRoutes[httpKey]; ok {
		t.Fatalf("did not expect passthrough route when no redirect rules exist")
	}

	origCtx := eIR.HTTPRoutes[key]
	if origCtx.Spec.ParentRefs[0].Port != nil {
		t.Fatalf("expected original route parentRef port to remain nil, got %v", *origCtx.Spec.ParentRefs[0].Port)
	}
}

func TestAddDefaultSSLRedirect_threeRulesMixed(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	parentRefs := []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}}

	ingEnabled := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-enabled",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}
	ingDisabled := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-disabled",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	pathA := "/a"
	pathB := "/b"
	pathC := "/c"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: append([]gatewayv1.ParentReference(nil), parentRefs...),
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathC}}}},
			},
		},
	}

	// Rule 0 (/a) -> enabled, Rule 1 (/b) -> disabled, Rule 2 (/c) -> enabled
	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingEnabled}},
			{{Ingress: &ingDisabled}},
			{{Ingress: &ingEnabled}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingEnabled, ingDisabled}, &pIR, &eIR)

	// Redirect route should have 2 rules (/a and /c)
	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route %v to be created", redirectKey)
	}
	if len(redirectCtx.Spec.Rules) != 2 {
		t.Fatalf("expected 2 redirect rules, got %d", len(redirectCtx.Spec.Rules))
	}
	if *redirectCtx.Spec.Rules[0].Matches[0].Path.Value != "/a" {
		t.Fatalf("expected first redirect rule to match /a, got %s", *redirectCtx.Spec.Rules[0].Matches[0].Path.Value)
	}
	if *redirectCtx.Spec.Rules[1].Matches[0].Path.Value != "/c" {
		t.Fatalf("expected second redirect rule to match /c, got %s", *redirectCtx.Spec.Rules[1].Matches[0].Path.Value)
	}

	// Passthrough route should have 1 rule (/b)
	httpKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-http"}
	httpCtx, ok := eIR.HTTPRoutes[httpKey]
	if !ok {
		t.Fatalf("expected passthrough route %v to be created", httpKey)
	}
	if len(httpCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 passthrough rule, got %d", len(httpCtx.Spec.Rules))
	}
	if *httpCtx.Spec.Rules[0].Matches[0].Path.Value != "/b" {
		t.Fatalf("expected passthrough rule to match /b, got %s", *httpCtx.Spec.Rules[0].Matches[0].Path.Value)
	}
}

func TestAddDefaultSSLRedirect_canarySourceIgnored(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	// Primary ingress disables SSL redirect
	ingPrimary := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-primary",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	// Canary ingress enables SSL redirect — should be ignored
	ingCanary := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-canary",
			Annotations: map[string]string{
				CanaryAnnotation:      "true",
				SSLRedirectAnnotation: "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	pathA := "/a"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
			},
		},
	}

	// Rule has both canary and primary sources; primary disables redirect
	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{
				{Ingress: &ingCanary},
				{Ingress: &ingPrimary},
			},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingPrimary, ingCanary}, &pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	if _, ok := eIR.HTTPRoutes[redirectKey]; ok {
		t.Fatalf("did not expect redirect route — primary ingress has ssl-redirect=false, canary should be ignored")
	}
}

func TestAddDefaultSSLRedirect_multipleParentRefs(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{Name: gatewayv1.ObjectName("gw1")},
					{Name: gatewayv1.ObjectName("gw2")},
				},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules:     []gatewayv1.HTTPRouteRule{{Matches: []gatewayv1.HTTPRouteMatch{{}}}},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ing}, &pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route to be created")
	}

	// Both parent refs on the redirect route should have port 80
	if len(redirectCtx.Spec.ParentRefs) != 2 {
		t.Fatalf("expected 2 parentRefs on redirect route, got %d", len(redirectCtx.Spec.ParentRefs))
	}
	for i, ref := range redirectCtx.Spec.ParentRefs {
		if ref.Port == nil || *ref.Port != 80 {
			t.Fatalf("redirect route parentRef[%d] expected port 80, got %v", i, ref.Port)
		}
	}

	// Both parent refs on the original route should have port 443
	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 2 {
		t.Fatalf("expected 2 parentRefs on original route, got %d", len(origCtx.Spec.ParentRefs))
	}
	for i, ref := range origCtx.Spec.ParentRefs {
		if ref.Port == nil || *ref.Port != 443 {
			t.Fatalf("original route parentRef[%d] expected port 443, got %v", i, ref.Port)
		}
	}
}

func TestAddDefaultSSLRedirect_mixedTLSAndNoTLSRules(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	// Ingress with TLS configured (default ssl-redirect=true)
	ingWithTLS := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-tls",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	// Ingress without TLS — but since the hostname has TLS from another ingress,
	// ssl-redirect should still apply (matching real ingress-nginx behavior where
	// TLS is merged at the server/hostname level).
	ingNoTLS := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-no-tls",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{},
	}

	pathA := "/a"
	pathB := "/b"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
			},
		},
	}

	// Rule 0 (/a) -> has TLS, Rule 1 (/b) -> no TLS on its own ingress
	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingWithTLS}},
			{{Ingress: &ingNoTLS}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingWithTLS, ingNoTLS}, &pIR, &eIR)

	// Both /a and /b should get redirects because the hostname has TLS
	// (from ingWithTLS), matching ingress-nginx's hostname-level TLS merging.
	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route for hostname with TLS")
	}
	if len(redirectCtx.Spec.Rules) != 2 {
		t.Fatalf("expected 2 redirect rules (both /a and /b), got %d", len(redirectCtx.Spec.Rules))
	}
	if *redirectCtx.Spec.Rules[0].Matches[0].Path.Value != "/a" {
		t.Fatalf("expected first redirect rule to match /a, got %s", *redirectCtx.Spec.Rules[0].Matches[0].Path.Value)
	}
	if *redirectCtx.Spec.Rules[1].Matches[0].Path.Value != "/b" {
		t.Fatalf("expected second redirect rule to match /b, got %s", *redirectCtx.Spec.Rules[1].Matches[0].Path.Value)
	}
}

func TestAddDefaultSSLRedirect_noTLS(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ing}, &pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	if _, ok := eIR.HTTPRoutes[redirectKey]; ok {
		t.Fatalf("did not expect redirect route %v to be added", redirectKey)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parentRef, got %#v", origCtx.Spec.ParentRefs)
	}
	if origCtx.Spec.ParentRefs[0].Port != nil {
		t.Fatalf("expected original route parentRef port to remain nil, got %#v", origCtx.Spec.ParentRefs[0].Port)
	}
}

func TestAddDefaultSSLRedirect_crossIngressTLSWithOptOut(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ingWithTLS := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-tls",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	// Ingress B has no TLS but explicitly opts out of ssl-redirect.
	// Even though the hostname has TLS, this path should not redirect.
	ingNoTLSOptOut := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-no-tls-optout",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{},
	}

	pathA := "/a"
	pathB := "/b"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
			},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingWithTLS}},
			{{Ingress: &ingNoTLSOptOut}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingWithTLS, ingNoTLSOptOut}, &pIR, &eIR)

	// /a should redirect (TLS ingress, default enabled)
	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route to be created")
	}
	if len(redirectCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 redirect rule, got %d", len(redirectCtx.Spec.Rules))
	}
	if *redirectCtx.Spec.Rules[0].Matches[0].Path.Value != "/a" {
		t.Fatalf("expected redirect rule to match /a, got %s", *redirectCtx.Spec.Rules[0].Matches[0].Path.Value)
	}

	// /b should be a passthrough on port 80 (explicit ssl-redirect=false)
	httpKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-http"}
	httpCtx, ok := eIR.HTTPRoutes[httpKey]
	if !ok {
		t.Fatalf("expected passthrough route for /b")
	}
	if len(httpCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 passthrough rule, got %d", len(httpCtx.Spec.Rules))
	}
	if *httpCtx.Spec.Rules[0].Matches[0].Path.Value != "/b" {
		t.Fatalf("expected passthrough rule to match /b, got %s", *httpCtx.Spec.Rules[0].Matches[0].Path.Value)
	}
}

func TestAddDefaultSSLRedirect_crossIngressTLSThreeWayMixed(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	parentRefs := []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}}

	ingWithTLS := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-tls",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret", Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}

	ingNoTLSOptOut := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-no-tls-optout",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{},
	}

	// Ingress C: no TLS, default ssl-redirect (inherits from hostname)
	ingNoTLSDefault := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-no-tls-default",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{},
	}

	pathA := "/a"
	pathB := "/b"
	pathC := "/c"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: append([]gatewayv1.ParentReference(nil), parentRefs...),
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathC}}}},
			},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingWithTLS}},
			{{Ingress: &ingNoTLSOptOut}},
			{{Ingress: &ingNoTLSDefault}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingWithTLS, ingNoTLSOptOut, ingNoTLSDefault}, &pIR, &eIR)

	// /a and /c should redirect; /b should not (explicit opt-out)
	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route to be created")
	}
	if len(redirectCtx.Spec.Rules) != 2 {
		t.Fatalf("expected 2 redirect rules, got %d", len(redirectCtx.Spec.Rules))
	}
	if *redirectCtx.Spec.Rules[0].Matches[0].Path.Value != "/a" {
		t.Fatalf("expected first redirect rule to match /a, got %s", *redirectCtx.Spec.Rules[0].Matches[0].Path.Value)
	}
	if *redirectCtx.Spec.Rules[1].Matches[0].Path.Value != "/c" {
		t.Fatalf("expected second redirect rule to match /c, got %s", *redirectCtx.Spec.Rules[1].Matches[0].Path.Value)
	}

	// /b should be a passthrough on port 80
	httpKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-http"}
	httpCtx, ok := eIR.HTTPRoutes[httpKey]
	if !ok {
		t.Fatalf("expected passthrough route for /b")
	}
	if len(httpCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 passthrough rule, got %d", len(httpCtx.Spec.Rules))
	}
	if *httpCtx.Spec.Rules[0].Matches[0].Path.Value != "/b" {
		t.Fatalf("expected passthrough rule to match /b, got %s", *httpCtx.Spec.Rules[0].Matches[0].Path.Value)
	}
}

func TestAddDefaultSSLRedirect_allIngressesNoTLS(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ingA := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-a",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{},
	}
	ingB := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing-b",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{},
	}

	pathA := "/a"
	pathB := "/b"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
			},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingA}},
			{{Ingress: &ingB}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect([]networkingv1.Ingress{ingA, ingB}, &pIR, &eIR)

	// No ingress has TLS, so no redirect should happen at all
	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	if _, ok := eIR.HTTPRoutes[redirectKey]; ok {
		t.Fatalf("did not expect redirect route when no ingress has TLS")
	}

	httpKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-http"}
	if _, ok := eIR.HTTPRoutes[httpKey]; ok {
		t.Fatalf("did not expect passthrough route when no ingress has TLS")
	}

	origCtx := eIR.HTTPRoutes[key]
	if origCtx.Spec.ParentRefs[0].Port != nil {
		t.Fatalf("expected original route parentRef port to remain nil, got %v", *origCtx.Spec.ParentRefs[0].Port)
	}
}
