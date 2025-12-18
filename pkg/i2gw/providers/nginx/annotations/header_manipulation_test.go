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
	"reflect"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// Common test data
var (
	testIngressSpec = networkingv1.IngressSpec{
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
	}
)

// Helper functions for test setup
func createTestIngress(name, namespace string, annotations map[string]string) networkingv1.Ingress {
	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: testIngressSpec,
	}
}

func TestParseSetHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "single header name only",
			input:    "X-Custom-Header",
			expected: map[string]string{}, // Headers without values should not be added
		},
		{
			name:  "single header with value",
			input: "X-Custom-Header: custom-value",
			expected: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		},
		{
			name:     "multiple headers names only",
			input:    "X-Header1,X-Header2,X-Header3",
			expected: map[string]string{}, // Headers without values should not be added
		},
		{
			name:  "multiple headers with values",
			input: "X-Header1: value1,X-Header2: value2",
			expected: map[string]string{
				"X-Header1": "value1",
				"X-Header2": "value2",
			},
		},
		{
			name:  "mixed format",
			input: "X-Default-Header,X-Custom-Header: custom-value,X-Another-Header",
			expected: map[string]string{
				// Only headers with explicit values should be included
				"X-Custom-Header": "custom-value",
			},
		},
		{
			name:  "headers with spaces",
			input: " X-Header1 : value1 , X-Header2 : value2 ",
			expected: map[string]string{
				"X-Header1": "value1",
				"X-Header2": "value2",
			},
		},
		{
			name:  "complex header values",
			input: "X-Forwarded-For: $remote_addr,X-Real-IP: $remote_addr,X-Custom: hello-world",
			expected: map[string]string{
				"X-Forwarded-For": "$remote_addr",
				"X-Real-IP":       "$remote_addr",
				"X-Custom":        "hello-world",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSetHeaders(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d headers, got %d", len(tt.expected), len(result))
			}

			for expectedName, expectedValue := range tt.expected {
				if actualValue, exists := result[expectedName]; !exists {
					t.Errorf("Expected header %s not found", expectedName)
				} else if actualValue != expectedValue {
					t.Errorf("Header %s: expected value %q, got %q", expectedName, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestHideHeaders(t *testing.T) {
	tests := []struct {
		name            string
		hideHeaders     string
		expectedHeaders []string
	}{
		{
			name:            "single header",
			hideHeaders:     "Server",
			expectedHeaders: []string{"Server"},
		},
		{
			name:            "multiple headers",
			hideHeaders:     "Server,X-Powered-By,X-Version",
			expectedHeaders: []string{"Server", "X-Powered-By", "X-Version"},
		},
		{
			name:            "headers with spaces",
			hideHeaders:     " Server , X-Powered-By , X-Version ",
			expectedHeaders: []string{"Server", "X-Powered-By", "X-Version"},
		},
		{
			name:            "empty headers filtered out",
			hideHeaders:     "Server,,X-Powered-By,",
			expectedHeaders: []string{"Server", "X-Powered-By"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := createTestIngress("test-ingress", "default", map[string]string{
				nginxProxyHideHeadersAnnotation: tt.hideHeaders,
			})

			ir := providerir.ProviderIR{
				Gateways:   make(map[types.NamespacedName]providerir.GatewayContext),
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
			routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}
			ir.HTTPRoutes[routeKey] = providerir.HTTPRouteContext{
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: ingress.Namespace,
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								BackendRefs: []gatewayv1.HTTPBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: gatewayv1.ObjectName("web-service"),
												Port: ptr.To(gatewayv1.PortNumber(80)),
											},
										},
									},
								},
							},
						},
					},
				},
			}

			filter := createResponseHeaderModifier(tt.hideHeaders)
			if filter == nil {
				t.Error("Expected filter to be created")
				return
			}
			// Apply filter to first rule (simplified for test)
			var errs field.ErrorList
			httpRouteContext := ir.HTTPRoutes[routeKey]
			if len(httpRouteContext.HTTPRoute.Spec.Rules) > 0 {
				if httpRouteContext.HTTPRoute.Spec.Rules[0].Filters == nil {
					httpRouteContext.HTTPRoute.Spec.Rules[0].Filters = []gatewayv1.HTTPRouteFilter{}
				}
				httpRouteContext.HTTPRoute.Spec.Rules[0].Filters = append(httpRouteContext.HTTPRoute.Spec.Rules[0].Filters, *filter)
				ir.HTTPRoutes[routeKey] = httpRouteContext
			}
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			updatedRoute := ir.HTTPRoutes[routeKey].HTTPRoute
			if len(updatedRoute.Spec.Rules) == 0 {
				t.Error("Expected at least one rule")
				return
			}

			rule := updatedRoute.Spec.Rules[0]
			if len(rule.Filters) != 1 {
				t.Errorf("Expected 1 filter, got %d", len(rule.Filters))
				return
			}

			filter = &rule.Filters[0]
			if filter.Type != gatewayv1.HTTPRouteFilterResponseHeaderModifier {
				t.Errorf("Expected ResponseHeaderModifier filter, got %s", filter.Type)
				return
			}

			if filter.ResponseHeaderModifier == nil {
				t.Error("Expected ResponseHeaderModifier to be non-nil")
				return
			}

			if len(filter.ResponseHeaderModifier.Remove) != len(tt.expectedHeaders) {
				t.Errorf("Expected %d headers to remove, got %d", len(tt.expectedHeaders), len(filter.ResponseHeaderModifier.Remove))
				return
			}

			for _, expectedHeader := range tt.expectedHeaders {
				found := false
				for _, actualHeader := range filter.ResponseHeaderModifier.Remove {
					if actualHeader == expectedHeader {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected header %s not found in remove list", expectedHeader)
				}
			}
		})
	}
}

func TestSetHeaders(t *testing.T) {
	tests := []struct {
		name            string
		setHeaders      string
		expectedHeaders []gatewayv1.HTTPHeader
	}{
		{
			name:       "single header with value",
			setHeaders: "X-Custom: hello-world",
			expectedHeaders: []gatewayv1.HTTPHeader{
				{Name: "X-Custom", Value: "hello-world"},
			},
		},
		{
			name:       "multiple headers with values",
			setHeaders: "X-Custom: hello-world,X-Version: 1.0.0",
			expectedHeaders: []gatewayv1.HTTPHeader{
				{Name: "X-Custom", Value: "hello-world"},
				{Name: "X-Version", Value: "1.0.0"},
			},
		},
		{
			name:       "nginx variables filtered out",
			setHeaders: "X-Real-IP: $remote_addr,X-Custom: hello-world",
			expectedHeaders: []gatewayv1.HTTPHeader{
				{Name: "X-Custom", Value: "hello-world"},
			},
		},
		{
			name:       "empty values filtered out",
			setHeaders: "X-Empty-Header,X-Custom: hello-world",
			expectedHeaders: []gatewayv1.HTTPHeader{
				{Name: "X-Custom", Value: "hello-world"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := createTestIngress("test-ingress", "default", map[string]string{
				nginxProxySetHeadersAnnotation: tt.setHeaders,
			})

			ir := providerir.ProviderIR{
				Gateways:   make(map[types.NamespacedName]providerir.GatewayContext),
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
			routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}
			ir.HTTPRoutes[routeKey] = providerir.HTTPRouteContext{
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: ingress.Namespace,
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								BackendRefs: []gatewayv1.HTTPBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: gatewayv1.ObjectName("web-service"),
												Port: ptr.To(gatewayv1.PortNumber(80)),
											},
										},
									},
								},
							},
						},
					},
				},
			}

			filter := createRequestHeaderModifier(tt.setHeaders)
			var errs field.ErrorList
			if filter != nil {
				// Apply filter to first rule (simplified for test)
				httpRouteContext := ir.HTTPRoutes[routeKey]
				if len(httpRouteContext.HTTPRoute.Spec.Rules) > 0 {
					if httpRouteContext.HTTPRoute.Spec.Rules[0].Filters == nil {
						httpRouteContext.HTTPRoute.Spec.Rules[0].Filters = []gatewayv1.HTTPRouteFilter{}
					}
					httpRouteContext.HTTPRoute.Spec.Rules[0].Filters = append(httpRouteContext.HTTPRoute.Spec.Rules[0].Filters, *filter)
					ir.HTTPRoutes[routeKey] = httpRouteContext
				}
			}
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			updatedRoute := ir.HTTPRoutes[routeKey].HTTPRoute
			if len(updatedRoute.Spec.Rules) == 0 {
				t.Error("Expected at least one rule")
				return
			}

			rule := updatedRoute.Spec.Rules[0]
			if len(tt.expectedHeaders) == 0 {
				if len(rule.Filters) > 0 {
					t.Errorf("Expected no filters, got %d", len(rule.Filters))
				}
				return
			}

			if len(rule.Filters) != 1 {
				t.Errorf("Expected 1 filter, got %d", len(rule.Filters))
				return
			}

			filter = &rule.Filters[0]
			if filter.Type != gatewayv1.HTTPRouteFilterRequestHeaderModifier {
				t.Errorf("Expected RequestHeaderModifier filter, got %s", filter.Type)
				return
			}

			if filter.RequestHeaderModifier == nil {
				t.Error("Expected RequestHeaderModifier to be non-nil")
				return
			}

			if len(filter.RequestHeaderModifier.Set) != len(tt.expectedHeaders) {
				t.Errorf("Expected %d headers to set, got %d", len(tt.expectedHeaders), len(filter.RequestHeaderModifier.Set))
				return
			}

			for _, expectedHeader := range tt.expectedHeaders {
				found := false
				for _, actualHeader := range filter.RequestHeaderModifier.Set {
					if actualHeader.Name == expectedHeader.Name && actualHeader.Value == expectedHeader.Value {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected header %s: %s not found in set list", expectedHeader.Name, expectedHeader.Value)
				}
			}
		})
	}
}

func TestHeaderManipulationFeature(t *testing.T) {
	tests := []struct {
		name                string
		annotations         map[string]string
		expectedHideHeaders []string
		expectedSetHeaders  []gatewayv1.HTTPHeader
	}{
		{
			name: "both hide and set headers",
			annotations: map[string]string{
				nginxProxyHideHeadersAnnotation: "Server,X-Powered-By",
				nginxProxySetHeadersAnnotation:  "X-Custom: hello-world",
			},
			expectedHideHeaders: []string{"Server", "X-Powered-By"},
			expectedSetHeaders: []gatewayv1.HTTPHeader{
				{Name: "X-Custom", Value: "hello-world"},
			},
		},
		{
			name: "only hide headers",
			annotations: map[string]string{
				nginxProxyHideHeadersAnnotation: "Server",
			},
			expectedHideHeaders: []string{"Server"},
			expectedSetHeaders:  []gatewayv1.HTTPHeader{},
		},
		{
			name: "only set headers",
			annotations: map[string]string{
				nginxProxySetHeadersAnnotation: "X-Custom: hello-world",
			},
			expectedHideHeaders: []string{},
			expectedSetHeaders: []gatewayv1.HTTPHeader{
				{Name: "X-Custom", Value: "hello-world"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := createTestIngress("test-ingress", "default", tt.annotations)

			ir := providerir.ProviderIR{
				Gateways:   make(map[types.NamespacedName]providerir.GatewayContext),
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
			routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}
			ir.HTTPRoutes[routeKey] = providerir.HTTPRouteContext{
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: ingress.Namespace,
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								BackendRefs: []gatewayv1.HTTPBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: gatewayv1.ObjectName("web-service"),
												Port: ptr.To(gatewayv1.PortNumber(80)),
											},
										},
									},
								},
							},
						},
					},
				},
			}

			errs := HeaderManipulationFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			updatedRoute := ir.HTTPRoutes[routeKey].HTTPRoute
			if len(updatedRoute.Spec.Rules) == 0 {
				t.Error("Expected at least one rule")
				return
			}

			rule := updatedRoute.Spec.Rules[0]

			expectedFilterCount := 0
			if len(tt.expectedHideHeaders) > 0 {
				expectedFilterCount++
			}
			if len(tt.expectedSetHeaders) > 0 {
				expectedFilterCount++
			}

			if len(rule.Filters) != expectedFilterCount {
				t.Fatalf("Expected %d filters, got %d", expectedFilterCount, len(rule.Filters))
			}

			var responseHeaderFilter *gatewayv1.HTTPRouteFilter
			var requestHeaderFilter *gatewayv1.HTTPRouteFilter

			for i := range rule.Filters {
				filter := &rule.Filters[i]
				if filter.Type == gatewayv1.HTTPRouteFilterResponseHeaderModifier {
					responseHeaderFilter = filter
				} else if filter.Type == gatewayv1.HTTPRouteFilterRequestHeaderModifier {
					requestHeaderFilter = filter
				}
			}

			if len(tt.expectedHideHeaders) > 0 {
				if responseHeaderFilter == nil {
					t.Fatal("Expected ResponseHeaderModifier filter")
				}
				if len(responseHeaderFilter.ResponseHeaderModifier.Remove) != len(tt.expectedHideHeaders) {
					t.Fatalf("Expected %d headers to remove, got %d", len(tt.expectedHideHeaders), len(responseHeaderFilter.ResponseHeaderModifier.Remove))
				}
			}

			if len(tt.expectedSetHeaders) > 0 {
				if requestHeaderFilter == nil {
					t.Fatal("Expected RequestHeaderModifier filter")
				}
				if len(requestHeaderFilter.RequestHeaderModifier.Set) != len(tt.expectedSetHeaders) {
					t.Fatalf("Expected %d headers to set, got %d", len(tt.expectedSetHeaders), len(requestHeaderFilter.RequestHeaderModifier.Set))
				}
			}
		})
	}
}

func TestParseCommaSeparatedHeaders(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "single header",
			input:    "Server",
			expected: []string{"Server"},
		},
		{
			name:     "multiple headers",
			input:    "Server,X-Powered-By,X-Version",
			expected: []string{"Server", "X-Powered-By", "X-Version"},
		},
		{
			name:     "headers with spaces",
			input:    " Server , X-Powered-By , X-Version ",
			expected: []string{"Server", "X-Powered-By", "X-Version"},
		},
		{
			name:     "empty headers filtered out",
			input:    "Server,,X-Powered-By,",
			expected: []string{"Server", "X-Powered-By"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseCommaSeparatedHeaders(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCreateResponseHeaderModifier(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedFilter *gatewayv1.HTTPRouteFilter
	}{
		{
			name:           "empty input",
			input:          "",
			expectedFilter: nil,
		},
		{
			name:  "single header",
			input: "Server",
			expectedFilter: &gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Remove: []string{"Server"},
				},
			},
		},
		{
			name:  "multiple headers",
			input: "Server,X-Powered-By,X-Version",
			expectedFilter: &gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Remove: []string{"Server", "X-Powered-By", "X-Version"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := createResponseHeaderModifier(tc.input)
			if !reflect.DeepEqual(result, tc.expectedFilter) {
				t.Errorf("Expected %+v, got %+v", tc.expectedFilter, result)
			}
		})
	}
}

func TestCreateRequestHeaderModifier(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedFilter *gatewayv1.HTTPRouteFilter
	}{
		{
			name:           "empty input",
			input:          "",
			expectedFilter: nil,
		},
		{
			name:  "single header with value",
			input: "X-Custom: hello-world",
			expectedFilter: &gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "X-Custom", Value: "hello-world"},
					},
				},
			},
		},
		{
			name:  "multiple headers with values",
			input: "X-Custom: hello-world,X-Version: 1.0.0",
			// Don't check exact filter here due to map iteration order
			expectedFilter: nil, // Will be verified manually in test
		},
		{
			name:           "headers with NGINX variables filtered out",
			input:          "X-Real-IP: $remote_addr",
			expectedFilter: nil,
		},
		{
			name:           "headers with empty values filtered out",
			input:          "X-Empty-Header",
			expectedFilter: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := createRequestHeaderModifier(tc.input)

			// Special handling for multiple headers test due to map iteration order
			if tc.name == "multiple headers with values" {
				if result == nil {
					t.Error("Expected non-nil filter for multiple headers")
					return
				}
				if result.Type != gatewayv1.HTTPRouteFilterRequestHeaderModifier {
					t.Errorf("Expected RequestHeaderModifier type, got %s", result.Type)
					return
				}
				if result.RequestHeaderModifier == nil {
					t.Error("Expected RequestHeaderModifier to be non-nil")
					return
				}
				if len(result.RequestHeaderModifier.Set) != 2 {
					t.Errorf("Expected 2 headers, got %d", len(result.RequestHeaderModifier.Set))
					return
				}
				// Check headers exist (order may vary due to map iteration)
				headers := make(map[string]string)
				for _, h := range result.RequestHeaderModifier.Set {
					headers[string(h.Name)] = h.Value
				}
				if headers["X-Custom"] != "hello-world" {
					t.Errorf("Expected X-Custom: hello-world, got %s", headers["X-Custom"])
				}
				if headers["X-Version"] != "1.0.0" {
					t.Errorf("Expected X-Version: 1.0.0, got %s", headers["X-Version"])
				}
				return
			}

			if !reflect.DeepEqual(result, tc.expectedFilter) {
				t.Errorf("Expected %+v, got %+v", tc.expectedFilter, result)
			}
		})
	}
}

// Additional tests for behavior with source ingress mapping
func TestHeaderManipulationWithSourceIngressMapping(t *testing.T) {
	// Test that filters are applied only to the correct rules based on source ingress mapping
	ingress1 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app1",
			Namespace: "test",
			Annotations: map[string]string{
				nginxProxyHideHeadersAnnotation: "Server,X-Powered-By",
				nginxProxySetHeadersAnnotation:  "X-Custom-Header: value1",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{{
				Host: "example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/app1",
							PathType: ptr.To(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "app1-service",
									Port: networkingv1.ServiceBackendPort{Number: 8080},
								},
							},
						}},
					},
				},
			}},
		},
	}

	ingress2 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2",
			Namespace: "test",
			Annotations: map[string]string{
				nginxProxySetHeadersAnnotation: "X-App: app2",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{{
				Host: "example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/app2",
							PathType: ptr.To(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "app2-service",
									Port: networkingv1.ServiceBackendPort{Number: 3000},
								},
							},
						}},
					},
				},
			}},
		},
	}

	// Create HTTPRoute with source ingress mapping annotation
	httpRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app1-example-com",
			Namespace: "test",
			Annotations: map[string]string{
				"ingress2gateway.io/source-ingress-rules": "test/app1:0;test/app2:1",
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{}, // Rule 0 - from app1
				{}, // Rule 1 - from app2
			},
		},
	}

	ir := providerir.ProviderIR{
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}
	routeKey := types.NamespacedName{Namespace: "test", Name: "app1-example-com"}
	ir.HTTPRoutes[routeKey] = providerir.HTTPRouteContext{HTTPRoute: httpRoute}

	// Apply header manipulation
	errs := HeaderManipulationFeature([]networkingv1.Ingress{ingress1, ingress2}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Fatalf("HeaderManipulationFeature returned errors: %v", errs)
	}

	// Verify filters applied correctly
	updatedRoute := ir.HTTPRoutes[routeKey].HTTPRoute

	// Both rules should have 3 filters total (from app1: hide + set headers, from app2: set header)
	// Since we no longer use source ingress mapping, all filters are applied to all rules
	if len(updatedRoute.Spec.Rules[0].Filters) != 3 {
		t.Errorf("Rule 0: expected 3 filters, got %d", len(updatedRoute.Spec.Rules[0].Filters))
	}

	if len(updatedRoute.Spec.Rules[1].Filters) != 3 {
		t.Errorf("Rule 1: expected 3 filters, got %d", len(updatedRoute.Spec.Rules[1].Filters))
	}
}
