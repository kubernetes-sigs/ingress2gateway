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

func TestPathRegex(t *testing.T) {
	tests := []struct {
		name                string
		annotations         map[string]string
		expectedPathType    gatewayv1.PathMatchType
		expectedPathValue   string
		shouldModifyMatches bool
	}{
		{
			name: "true enables regex",
			annotations: map[string]string{
				"nginx.org/path-regex": "true",
			},
			expectedPathType:    gatewayv1.PathMatchRegularExpression,
			expectedPathValue:   "/api/.*",
			shouldModifyMatches: true,
		},
		{
			name: "case_sensitive enables regex",
			annotations: map[string]string{
				"nginx.org/path-regex": "case_sensitive",
			},
			expectedPathType:    gatewayv1.PathMatchRegularExpression,
			expectedPathValue:   "/api/.*",
			shouldModifyMatches: true,
		},
		{
			name: "case_insensitive enables regex",
			annotations: map[string]string{
				"nginx.org/path-regex": "case_insensitive",
			},
			expectedPathType:    gatewayv1.PathMatchRegularExpression,
			expectedPathValue:   "(?i)/api/.*",
			shouldModifyMatches: true,
		},
		{
			name: "exact enables exact matching",
			annotations: map[string]string{
				"nginx.org/path-regex": "exact",
			},
			expectedPathType:    gatewayv1.PathMatchExact,
			expectedPathValue:   "/api/.*",
			shouldModifyMatches: true,
		},
		{
			name: "false disables regex",
			annotations: map[string]string{
				"nginx.org/path-regex": "false",
			},
			expectedPathType:    gatewayv1.PathMatchPathPrefix,
			expectedPathValue:   "/api/.*",
			shouldModifyMatches: false,
		},
		{
			name: "missing annotation disables regex",
			annotations: map[string]string{
				"nginx.org/rewrites": "service=/api",
			},
			expectedPathType:    gatewayv1.PathMatchPathPrefix,
			expectedPathValue:   "/api/.*",
			shouldModifyMatches: false,
		},
		{
			name:                "no annotations disables regex",
			annotations:         map[string]string{},
			expectedPathType:    gatewayv1.PathMatchPathPrefix,
			expectedPathValue:   "/api/.*",
			shouldModifyMatches: false,
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
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/api/.*",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "api-service",
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

			ir := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
			routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

			httpRoute := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      routeName,
					Namespace: ingress.Namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/api/.*"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name:  gatewayv1.ObjectName("api-service"),
											Kind:  ptr.To(gatewayv1.Kind("Service")),
											Group: ptr.To(gatewayv1.Group("")),
											Port:  ptr.To(gatewayv1.PortNumber(80)),
										},
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

			errs := PathRegexFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			updatedRoute := ir.HTTPRoutes[routeKey]
			if len(updatedRoute.HTTPRoute.Spec.Rules) == 0 || len(updatedRoute.HTTPRoute.Spec.Rules[0].Matches) == 0 {
				t.Error("Expected HTTPRoute to have rules and matches")
				return
			}

			match := updatedRoute.HTTPRoute.Spec.Rules[0].Matches[0]
			if match.Path == nil {
				t.Error("Expected path match to exist")
				return
			}

			actualPathType := *match.Path.Type
			if actualPathType != tt.expectedPathType {
				t.Errorf("Expected path type %v, got %v", tt.expectedPathType, actualPathType)
			}

			if *match.Path.Value != tt.expectedPathValue {
				t.Errorf("Expected path value %v, got %v", tt.expectedPathValue, *match.Path.Value)
			}
		})
	}
}

func TestPathRegexMultipleMatches(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-multi-paths",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.org/path-regex": "true",
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
									Path: "/api/v1/.*",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-v1-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
								{
									Path: "/api/v2/.*",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-v2-service",
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

	ir := providerir.ProviderIR{
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}

	routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
	routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

	httpRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: ingress.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
								Value: ptr.To("/api/v1/.*"),
							},
						},
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
								Value: ptr.To("/api/v2/.*"),
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

	errs := PathRegexFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	updatedRoute := ir.HTTPRoutes[routeKey]
	matches := updatedRoute.HTTPRoute.Spec.Rules[0].Matches

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
		return
	}

	for i, match := range matches {
		if match.Path == nil {
			t.Errorf("Expected path match %d to exist", i)
			return
		}

		if *match.Path.Type != gatewayv1.PathMatchRegularExpression {
			t.Errorf("Expected match %d to have RegularExpression type, got %v", i, *match.Path.Type)
		}
	}
}

func TestPathRegexCaseInsensitiveNotification(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-insensitive",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.org/path-regex": "case_insensitive",
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
									Path: "/api/.*",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-service",
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

	ir := providerir.ProviderIR{
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}

	routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
	routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

	httpRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: ingress.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
								Value: ptr.To("/api/.*"),
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

	errs := PathRegexFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)

	// Should have no errors since we're using notifications now
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(errs))
		return
	}

	// Verify path type is still set correctly
	updatedRoute := ir.HTTPRoutes[routeKey]
	if *updatedRoute.HTTPRoute.Spec.Rules[0].Matches[0].Path.Type != gatewayv1.PathMatchRegularExpression {
		t.Errorf("Expected path type to be PathMatchRegularExpression")
	}

	// Note: Testing notifications requires access to the notification aggregator,
	// which is more complex to test in unit tests. The notification dispatch
	// is tested through integration tests.
}

func TestPathRegexCaseInsensitiveFlagInjection(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				nginxPathRegexAnnotation: "case_insensitive",
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
									Path:     "/api/v[0-9]+",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-service",
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

	// Create HTTPRoute first (simulating what common converter creates)
	routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
	routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

	originalPath := "/api/v[0-9]+"
	ir := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			routeKey: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: ingress.Namespace,
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
											Value: &originalPath,
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

	// Apply path regex feature
	errs := PathRegexFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	// Verify the path was modified
	updatedRoute := ir.HTTPRoutes[routeKey]
	if len(updatedRoute.HTTPRoute.Spec.Rules) == 0 {
		t.Error("Expected HTTPRoute to have rules")
		return
	}

	rule := updatedRoute.HTTPRoute.Spec.Rules[0]
	if len(rule.Matches) == 0 {
		t.Error("Expected HTTPRoute rule to have matches")
		return
	}

	match := rule.Matches[0]
	if match.Path == nil {
		t.Error("Expected HTTPRoute match to have path")
		return
	}

	// Verify path match type is regular expression
	if match.Path.Type == nil || *match.Path.Type != gatewayv1.PathMatchRegularExpression {
		t.Errorf("Expected PathMatchRegularExpression, got %v", match.Path.Type)
	}

	if match.Path.Value == nil {
		t.Error("Expected path value to be set")
		return
	}

	// Verify (?i) flag was injected
	expectedPath := "(?i)/api/v[0-9]+"
	if *match.Path.Value != expectedPath {
		t.Errorf("Expected path value '%s', got '%s'", expectedPath, *match.Path.Value)
	}
}

func TestPathRegexCaseInsensitiveFlagNotDuplicated(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				nginxPathRegexAnnotation: "case_insensitive",
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
									Path:     "/api/v[0-9]+",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-service",
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

	// Create HTTPRoute with path that already has (?i) flag
	routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
	routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}

	originalPath := "(?i)/api/v[0-9]+"
	ir := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			routeKey: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: ingress.Namespace,
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
											Value: &originalPath,
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

	// Apply path regex feature
	errs := PathRegexFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	// Verify the path was NOT duplicated
	updatedRoute := ir.HTTPRoutes[routeKey]
	match := updatedRoute.HTTPRoute.Spec.Rules[0].Matches[0]

	// Should still be the original path, not (?i)(?i)/api/v[0-9]+
	expectedPath := "(?i)/api/v[0-9]+"
	if *match.Path.Value != expectedPath {
		t.Errorf("Expected path value '%s', got '%s'", expectedPath, *match.Path.Value)
	}
}
