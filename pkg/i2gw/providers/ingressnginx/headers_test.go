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
	"testing"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHeaderModifierFeature(t *testing.T) {
	testCases := []struct {
		name            string
		ingress         networkingv1.Ingress
		expectedHeaders map[string]string
	}{
		{
			name: "upstream-vhost header",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-upstream-vhost",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/upstream-vhost": "internal.example.com",
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
			expectedHeaders: map[string]string{
				"Host": "internal.example.com",
			},
		},
		{
			name: "connection-proxy-header",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-connection-header",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/connection-proxy-header": "keep-alive",
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
			expectedHeaders: map[string]string{
				"Connection": "keep-alive",
			},
		},
		{
			name: "multiple headers",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-multiple",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/upstream-vhost":          "backend.local",
						"nginx.ingress.kubernetes.io/connection-proxy-header": "keep-alive",
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
			expectedHeaders: map[string]string{
				"Host":               "backend.local",
				"Connection":         "keep-alive",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			// Replicate IR setup
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

			errs := headerModifierFeature([]networkingv1.Ingress{tc.ingress}, nil, &ir)
			if len(errs) > 0 {
				t.Fatalf("Expected no errors, got %v", errs)
			}

			result := ir.HTTPRoutes[key]
			rules := result.HTTPRoute.Spec.Rules
			if len(rules) != 1 {
				t.Fatalf("Expected 1 rule, got %d", len(rules))
			}

			foundHeaderFilter := false
			var headerFilter *gatewayv1.HTTPHeaderFilter

			for _, f := range rules[0].Filters {
				if f.Type == gatewayv1.HTTPRouteFilterRequestHeaderModifier {
					foundHeaderFilter = true
					headerFilter = f.RequestHeaderModifier
					break
				}
			}

			if len(tc.expectedHeaders) == 0 {
				if foundHeaderFilter {
					t.Fatalf("Expected no RequestHeaderModifier filter, but found one")
				}
				return // Success for empty check
			}

			if !foundHeaderFilter {
				t.Fatalf("Expected RequestHeaderModifier filter to be applied")
			}

			for k, v := range tc.expectedHeaders {
				found := false
				for _, h := range headerFilter.Set {
					if string(h.Name) == k {
						found = true
						if h.Value != v {
							t.Errorf("Expected header %s to have value %s, got %s", k, v, h.Value)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected header %s to be set", k)
				}
			}
		})
	}
}
