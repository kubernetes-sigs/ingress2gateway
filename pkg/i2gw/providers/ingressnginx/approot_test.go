/*
Copyright The Kubernetes Authors.

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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_appRootFeature(t *testing.T) {
	tests := []struct {
		name              string
		ingress           networkingv1.Ingress
		initialHTTPRoute  *gatewayv1.HTTPRoute
		expectedHTTPRoute *gatewayv1.HTTPRoute
	}{
		{
			name: "app-root annotation adds redirect rule",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						AppRootAnnotation: "/app1",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "approot.bar.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "http-svc",
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
					Name:      "test-ingress-approot-bar-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"approot.bar.com"},
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
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "http-svc", Port: ptr.To(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-approot-bar-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"approot.bar.com"},
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
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "http-svc", Port: ptr.To(gatewayv1.PortNumber(80))}}},
							},
						},
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchExact),
										Value: ptr.To("/"),
									},
								},
							},
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterRequestRedirect,
									RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
										Path: &gatewayv1.HTTPPathModifier{
											Type:            gatewayv1.FullPathHTTPPathModifier,
											ReplaceFullPath: ptr.To("/app1"),
										},
										StatusCode: ptr.To(302),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "no app-root annotation leaves route unchanged",
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
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/"),
									},
								},
							},
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
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "normal", Port: ptr.To(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
		},
		{
			name: "app-root annotation ignored for non-root path",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						AppRootAnnotation: "/app1",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "nonroot.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/api",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "api-svc",
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
					Name:      "test-ingress-nonroot-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"nonroot.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/api"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "api-svc", Port: ptr.To(gatewayv1.PortNumber(8080))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-nonroot-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"nonroot.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/api"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "api-svc", Port: ptr.To(gatewayv1.PortNumber(8080))}}},
							},
						},
					},
				},
			},
		},
		{
			name: "empty app-root annotation is ignored",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						AppRootAnnotation: "",
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
			},
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-empty-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"empty.com"},
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
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "empty", Port: ptr.To(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-empty-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"empty.com"},
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
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "empty", Port: ptr.To(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{},
			}

			if tt.initialHTTPRoute != nil {
				routeKey := types.NamespacedName{
					Namespace: tt.initialHTTPRoute.Namespace,
					Name:      tt.initialHTTPRoute.Name,
				}
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

			errs := appRootFeature(notifications.NoopNotify, []networkingv1.Ingress{tt.ingress}, nil, &ir)
			if len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
			}

			if tt.expectedHTTPRoute != nil {
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

				if len(actualRoute.Spec.Rules) != len(tt.expectedHTTPRoute.Spec.Rules) {
					t.Errorf("Expected %d rules, got %d", len(tt.expectedHTTPRoute.Spec.Rules), len(actualRoute.Spec.Rules))
					return
				}

				for i, expectedRule := range tt.expectedHTTPRoute.Spec.Rules {
					actualRule := actualRoute.Spec.Rules[i]

					// Verify matches.
					if len(actualRule.Matches) != len(expectedRule.Matches) {
						t.Errorf("Rule %d: expected %d matches, got %d", i, len(expectedRule.Matches), len(actualRule.Matches))
						continue
					}
					for j, expectedMatch := range expectedRule.Matches {
						actualMatch := actualRule.Matches[j]
						if expectedMatch.Path != nil {
							if actualMatch.Path == nil {
								t.Errorf("Rule %d, Match %d: expected path to be set", i, j)
								continue
							}
							if *actualMatch.Path.Type != *expectedMatch.Path.Type {
								t.Errorf("Rule %d, Match %d: expected path type %v, got %v", i, j, *expectedMatch.Path.Type, *actualMatch.Path.Type)
							}
							if *actualMatch.Path.Value != *expectedMatch.Path.Value {
								t.Errorf("Rule %d, Match %d: expected path value %s, got %s", i, j, *expectedMatch.Path.Value, *actualMatch.Path.Value)
							}
						}
					}

					// Verify filters.
					if len(actualRule.Filters) != len(expectedRule.Filters) {
						t.Errorf("Rule %d: expected %d filters, got %d", i, len(expectedRule.Filters), len(actualRule.Filters))
						continue
					}
					for j, expectedFilter := range expectedRule.Filters {
						actualFilter := actualRule.Filters[j]
						if actualFilter.Type != expectedFilter.Type {
							t.Errorf("Rule %d, Filter %d: expected type %v, got %v", i, j, expectedFilter.Type, actualFilter.Type)
						}
						if expectedFilter.RequestRedirect != nil {
							if actualFilter.RequestRedirect == nil {
								t.Errorf("Rule %d, Filter %d: expected RequestRedirect to be set", i, j)
								continue
							}
							if expectedFilter.RequestRedirect.StatusCode != nil {
								if actualFilter.RequestRedirect.StatusCode == nil || *actualFilter.RequestRedirect.StatusCode != *expectedFilter.RequestRedirect.StatusCode {
									t.Errorf("Rule %d, Filter %d: expected status code %d, got %v", i, j, *expectedFilter.RequestRedirect.StatusCode, actualFilter.RequestRedirect.StatusCode)
								}
							}
							if expectedFilter.RequestRedirect.Path != nil {
								if actualFilter.RequestRedirect.Path == nil {
									t.Errorf("Rule %d, Filter %d: expected path modifier to be set", i, j)
								} else {
									if actualFilter.RequestRedirect.Path.Type != expectedFilter.RequestRedirect.Path.Type {
										t.Errorf("Rule %d, Filter %d: expected path type %v, got %v", i, j, expectedFilter.RequestRedirect.Path.Type, actualFilter.RequestRedirect.Path.Type)
									}
									if expectedFilter.RequestRedirect.Path.ReplaceFullPath != nil {
										if actualFilter.RequestRedirect.Path.ReplaceFullPath == nil || *actualFilter.RequestRedirect.Path.ReplaceFullPath != *expectedFilter.RequestRedirect.Path.ReplaceFullPath {
											t.Errorf("Rule %d, Filter %d: expected ReplaceFullPath %s, got %v", i, j, *expectedFilter.RequestRedirect.Path.ReplaceFullPath, actualFilter.RequestRedirect.Path.ReplaceFullPath)
										}
									}
								}
							}
						}
					}

					// Verify backend refs.
					if len(actualRule.BackendRefs) != len(expectedRule.BackendRefs) {
						t.Errorf("Rule %d: expected %d backend refs, got %d", i, len(expectedRule.BackendRefs), len(actualRule.BackendRefs))
					}
				}
			}
		})
	}
}
