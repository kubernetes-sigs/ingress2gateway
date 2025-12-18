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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

func TestRewriteTarget(t *testing.T) {
	tests := []struct {
		name           string
		ingress        networkingv1.Ingress
		expectedFilter *gatewayv1.HTTPRouteFilter
	}{
		{
			name: "simple format",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.org/rewrites": "serviceName=web-service rewrite=/api/v1",
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
											Path: "/app",
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
			},
			expectedFilter: &gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterURLRewrite,
				URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
					Path: &gatewayv1.HTTPPathModifier{
						Type:               gatewayv1.PrefixMatchHTTPPathModifier,
						ReplacePrefixMatch: ptr.To("/api/v1"),
					},
				},
			},
		},
		{
			name: "NIC format",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-nic",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.org/rewrites": "serviceName=coffee rewrite=/coffee",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "coffee.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/app",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "coffee",
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
			expectedFilter: &gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterURLRewrite,
				URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
					Path: &gatewayv1.HTTPPathModifier{
						Type:               gatewayv1.PrefixMatchHTTPPathModifier,
						ReplacePrefixMatch: ptr.To("/coffee"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			routeName := common.RouteName(tt.ingress.Name, tt.ingress.Spec.Rules[0].Host)
			routeKey := types.NamespacedName{Namespace: tt.ingress.Namespace, Name: routeName}

			httpRoute := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      routeName,
					Namespace: tt.ingress.Namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/app"),
									},
								},
							},
						},
					},
				},
			}

			ir.HTTPRoutes[routeKey] = providerir.HTTPRouteContext{
				HTTPRoute: httpRoute,
			}

			errs := RewriteTargetFeature([]networkingv1.Ingress{tt.ingress}, nil, &ir, nil)
			if len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
				return
			}

			updatedRoute := ir.HTTPRoutes[routeKey]
			if len(updatedRoute.HTTPRoute.Spec.Rules) == 0 || len(updatedRoute.HTTPRoute.Spec.Rules[0].Filters) == 0 {
				t.Errorf("Expected filter to be added to HTTPRoute")
				return
			}

			filter := updatedRoute.HTTPRoute.Spec.Rules[0].Filters[0]
			if filter.Type != tt.expectedFilter.Type {
				t.Errorf("Expected filter type %v, got %v", tt.expectedFilter.Type, filter.Type)
			}

			if filter.URLRewrite == nil || filter.URLRewrite.Path == nil {
				t.Errorf("Expected URLRewrite filter with Path modifier")
				return
			}

			if *filter.URLRewrite.Path.ReplacePrefixMatch != *tt.expectedFilter.URLRewrite.Path.ReplacePrefixMatch {
				t.Errorf("Expected rewrite path %v, got %v",
					*tt.expectedFilter.URLRewrite.Path.ReplacePrefixMatch,
					*filter.URLRewrite.Path.ReplacePrefixMatch)
			}
		})
	}
}

func TestParseRewriteRules(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedRules map[string]string
	}{
		{
			name:  "single rule",
			input: "serviceName=coffee rewrite=/coffee",
			expectedRules: map[string]string{
				"coffee": "/coffee",
			},
		},
		{
			name:  "multiple rules",
			input: "serviceName=coffee rewrite=/coffee;serviceName=tea rewrite=/tea",
			expectedRules: map[string]string{
				"coffee": "/coffee",
				"tea":    "/tea",
			},
		},
		{
			name:  "rules with spaces",
			input: "serviceName=coffee rewrite=/coffee ; serviceName=tea rewrite=/tea ",
			expectedRules: map[string]string{
				"coffee": "/coffee",
				"tea":    "/tea",
			},
		},
		{
			name:          "empty input",
			input:         "",
			expectedRules: map[string]string{},
		},
		{
			name:          "invalid format",
			input:         "invalid-rule-without-equals",
			expectedRules: map[string]string{},
		},
		{
			name:  "complex path",
			input: "serviceName=api-service rewrite=/api/v2/users",
			expectedRules: map[string]string{
				"api-service": "/api/v2/users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRewriteRules(tt.input)

			if len(result) != len(tt.expectedRules) {
				t.Errorf("Expected %d rules, got %d", len(tt.expectedRules), len(result))
			}

			for expectedService, expectedPath := range tt.expectedRules {
				if actualPath, exists := result[expectedService]; !exists {
					t.Errorf("Expected service %s not found in result", expectedService)
				} else if actualPath != expectedPath {
					t.Errorf("Expected path %s for service %s, got %s", expectedPath, expectedService, actualPath)
				}
			}
		})
	}
}
