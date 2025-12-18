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

func TestHSTSFeature(t *testing.T) {
	tests := []struct {
		name               string
		annotations        map[string]string
		expectHSTS         bool
		expectedMaxAge     string
		expectedSubdomains bool
	}{
		{
			name: "HSTS enabled with defaults",
			annotations: map[string]string{
				nginxHSTSAnnotation: "true",
			},
			expectHSTS:         true,
			expectedMaxAge:     "31536000",
			expectedSubdomains: false,
		},
		{
			name: "HSTS with custom max-age",
			annotations: map[string]string{
				nginxHSTSAnnotation:       "true",
				nginxHSTSMaxAgeAnnotation: "86400",
			},
			expectHSTS:         true,
			expectedMaxAge:     "86400",
			expectedSubdomains: false,
		},
		{
			name: "HSTS with include subdomains",
			annotations: map[string]string{
				nginxHSTSAnnotation:                  "true",
				nginxHSTSIncludeSubdomainsAnnotation: "true",
			},
			expectHSTS:         true,
			expectedMaxAge:     "31536000",
			expectedSubdomains: true,
		},
		{
			name: "HSTS with all options",
			annotations: map[string]string{
				nginxHSTSAnnotation:                  "true",
				nginxHSTSMaxAgeAnnotation:            "604800",
				nginxHSTSIncludeSubdomainsAnnotation: "true",
			},
			expectHSTS:         true,
			expectedMaxAge:     "604800",
			expectedSubdomains: true,
		},
		{
			name: "HSTS disabled",
			annotations: map[string]string{
				nginxHSTSAnnotation: "false",
			},
			expectHSTS: false,
		},
		{
			name:        "no HSTS annotation",
			annotations: map[string]string{},
			expectHSTS:  false,
		},
		{
			name: "invalid max-age falls back to default",
			annotations: map[string]string{
				nginxHSTSAnnotation:       "true",
				nginxHSTSMaxAgeAnnotation: "invalid",
			},
			expectHSTS:         true,
			expectedMaxAge:     "31536000", // Should use default
			expectedSubdomains: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-ingress",
					Namespace:   "default",
					Annotations: tt.annotations,
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

			// Setup IR with existing HTTPRoute
			routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
			routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

			ir := providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					routeKey: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{
								Name:      routeName,
								Namespace: ingress.Namespace,
							},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{
										{
											Name: "nginx",
										},
									},
								},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "web-service",
														Port: ptr.To(gatewayv1.PortNumber(80)),
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

			// Execute
			errs := HSTSFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			// Verify results
			httpRoute := ir.HTTPRoutes[routeKey].HTTPRoute

			if !tt.expectHSTS {
				// Should not have added HSTS filter
				if len(httpRoute.Spec.Rules) > 0 && len(httpRoute.Spec.Rules[0].Filters) > 0 {
					for _, filter := range httpRoute.Spec.Rules[0].Filters {
						if filter.Type == gatewayv1.HTTPRouteFilterResponseHeaderModifier {
							if filter.ResponseHeaderModifier != nil {
								for _, header := range filter.ResponseHeaderModifier.Set {
									if header.Name == "Strict-Transport-Security" {
										t.Error("Expected no HSTS filter, but found one")
									}
								}
							}
						}
					}
				}
				return
			}

			// Verify HSTS filter was added
			if len(httpRoute.Spec.Rules) == 0 {
				t.Error("Expected HTTPRoute to have rules")
				return
			}

			rule := httpRoute.Spec.Rules[0]
			if len(rule.Filters) == 0 {
				t.Error("Expected HTTPRoute rule to have filters")
				return
			}

			// Find the HSTS filter
			var hstsFilter *gatewayv1.HTTPRouteFilter
			for i, filter := range rule.Filters {
				if filter.Type == gatewayv1.HTTPRouteFilterResponseHeaderModifier {
					if filter.ResponseHeaderModifier != nil {
						for _, header := range filter.ResponseHeaderModifier.Set {
							if header.Name == "Strict-Transport-Security" {
								hstsFilter = &rule.Filters[i]
								break
							}
						}
					}
				}
			}

			if hstsFilter == nil {
				t.Error("Expected HSTS ResponseHeaderModifier filter")
				return
			}

			// Verify the HSTS header value
			var hstsHeaderValue string
			for _, header := range hstsFilter.ResponseHeaderModifier.Set {
				if header.Name == "Strict-Transport-Security" {
					hstsHeaderValue = header.Value
					break
				}
			}

			expectedValue := buildHSTS(tt.expectedMaxAge, tt.expectedSubdomains)
			if hstsHeaderValue != expectedValue {
				t.Errorf("Expected HSTS header value %q, got %q", expectedValue, hstsHeaderValue)
			}
		})
	}
}

func TestBuildHSTS(t *testing.T) {
	tests := []struct {
		name              string
		maxAge            string
		includeSubdomains bool
		expectedValue     string
	}{
		{
			name:              "default settings",
			maxAge:            "31536000",
			includeSubdomains: false,
			expectedValue:     "max-age=31536000",
		},
		{
			name:              "with subdomains",
			maxAge:            "31536000",
			includeSubdomains: true,
			expectedValue:     "max-age=31536000; includeSubDomains",
		},
		{
			name:              "custom max-age",
			maxAge:            "86400",
			includeSubdomains: false,
			expectedValue:     "max-age=86400",
		},
		{
			name:              "custom max-age with subdomains",
			maxAge:            "604800",
			includeSubdomains: true,
			expectedValue:     "max-age=604800; includeSubDomains",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildHSTS(tt.maxAge, tt.includeSubdomains)
			if result != tt.expectedValue {
				t.Errorf("Expected %q, got %q", tt.expectedValue, result)
			}
		})
	}
}
