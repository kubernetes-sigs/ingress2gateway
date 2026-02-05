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

func TestApplyCorsToEmitterIR(t *testing.T) {
	testCases := []struct {
		name         string
		ingress      networkingv1.Ingress
		expectedCors *gatewayv1.HTTPCORSFilter
	}{
		{
			name: "enable-cors defaults",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cors-defaults",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/enable-cors": "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedCors: &gatewayv1.HTTPCORSFilter{
				AllowOrigins:     []gatewayv1.CORSOrigin{"*"},
				AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"},
				AllowHeaders:     []gatewayv1.HTTPHeaderName{"DNT", "Keep-Alive", "User-Agent", "X-Requested-With", "If-Modified-Since", "Cache-Control", "Content-Type", "Range", "Authorization"},
				AllowCredentials: ptr.To(true),
				MaxAge:           1728000,
			},
		},
		{
			name: "specific origin and expose headers",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cors-origin",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/enable-cors":         "true",
						"nginx.ingress.kubernetes.io/cors-allow-origin":   "https://foo.com, https://bar.com",
						"nginx.ingress.kubernetes.io/cors-expose-headers": "X-Exposed-1, X-Exposed-2",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedCors: &gatewayv1.HTTPCORSFilter{
				AllowOrigins:     []gatewayv1.CORSOrigin{"https://foo.com", "https://bar.com"},
				AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"},
				AllowHeaders:     []gatewayv1.HTTPHeaderName{"DNT", "Keep-Alive", "User-Agent", "X-Requested-With", "If-Modified-Since", "Cache-Control", "Content-Type", "Range", "Authorization"},
				ExposeHeaders:    []gatewayv1.HTTPHeaderName{"X-Exposed-1", "X-Exposed-2"},
				AllowCredentials: ptr.To(true),
				MaxAge:           1728000,
			},
		},
		{
			name: "explicit max age and false credentials",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cors-max-age",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/enable-cors":            "true",
						"nginx.ingress.kubernetes.io/cors-max-age":           "600",
						"nginx.ingress.kubernetes.io/cors-allow-credentials": "false",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedCors: &gatewayv1.HTTPCORSFilter{
				AllowOrigins:     []gatewayv1.CORSOrigin{"*"},
				AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"},
				AllowHeaders:     []gatewayv1.HTTPHeaderName{"DNT", "Keep-Alive", "User-Agent", "X-Requested-With", "If-Modified-Since", "Cache-Control", "Content-Type", "Range", "Authorization"},
				AllowCredentials: ptr.To(false),
				MaxAge:           600,
			},
		},
		{
			name: "cors disabled",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cors-disabled",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/enable-cors": "false",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedCors: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pIR := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}
			eIR := emitterir.EmitterIR{
				HTTPRoutes: make(map[types.NamespacedName]emitterir.HTTPRouteContext),
			}

			key := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: common.RouteName(tc.ingress.Name, "example.com")}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: tc.ingress.Namespace, Name: key.Name},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: ptr.To(gatewayv1.PathMatchPathPrefix), Value: ptr.To("/")}}},
						},
					},
				},
			}

			// Provider IR setup (for sources)
			pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
				HTTPRoute: route,
				RuleBackendSources: [][]providerir.BackendSource{{
					{Ingress: &tc.ingress},
				}},
			}

			// Emitter IR setup (target)
			eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{
				HTTPRoute: route,
			}

			errs := applyCorsToEmitterIR(pIR, &eIR)
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			result := eIR.HTTPRoutes[key]
			var cors *gatewayv1.HTTPCORSFilter
			if result.CorsPolicyByRuleIdx != nil {
				cors = result.CorsPolicyByRuleIdx[0]
			}

			if tc.expectedCors == nil {
				if cors != nil {
					t.Fatalf("Expected nil CORS, got %v", cors)
				}
				return
			}
			if cors == nil {
				t.Fatalf("Expected CORS policy, got nil")
			}

			// Validate Origins
			if len(cors.AllowOrigins) != len(tc.expectedCors.AllowOrigins) {
				t.Errorf("Expected %d origins, got %d", len(tc.expectedCors.AllowOrigins), len(cors.AllowOrigins))
			} else {
				for i, o := range tc.expectedCors.AllowOrigins {
					if o != cors.AllowOrigins[i] {
						t.Errorf("Origin mismatch at %d: expected %v, got %v", i, o, cors.AllowOrigins[i])
					}
				}
			}

			// Validate Methods
			if len(cors.AllowMethods) != len(tc.expectedCors.AllowMethods) {
				t.Errorf("Expected %d methods, got %d", len(tc.expectedCors.AllowMethods), len(cors.AllowMethods))
			} else {
				for i, m := range tc.expectedCors.AllowMethods {
					if m != cors.AllowMethods[i] {
						t.Errorf("Method mismatch at %d: expected %v, got %v", i, m, cors.AllowMethods[i])
					}
				}
			}

			// Validate Headers
			if len(cors.AllowHeaders) != len(tc.expectedCors.AllowHeaders) {
				t.Errorf("Expected %d headers, got %d", len(tc.expectedCors.AllowHeaders), len(cors.AllowHeaders))
			} else {
				for i, h := range tc.expectedCors.AllowHeaders {
					if h != cors.AllowHeaders[i] {
						t.Errorf("Header mismatch at %d: expected %v, got %v", i, h, cors.AllowHeaders[i])
					}
				}
			}

			// Validate Expose Headers
			if len(cors.ExposeHeaders) != len(tc.expectedCors.ExposeHeaders) {
				t.Errorf("Expected %d expose headers, got %d", len(tc.expectedCors.ExposeHeaders), len(cors.ExposeHeaders))
			} else {
				for i, h := range tc.expectedCors.ExposeHeaders {
					if h != cors.ExposeHeaders[i] {
						t.Errorf("Expose Header mismatch at %d: expected %v, got %v", i, h, cors.ExposeHeaders[i])
					}
				}
			}

			// Validate Credentials
			if tc.expectedCors.AllowCredentials != nil {
				if cors.AllowCredentials == nil || *cors.AllowCredentials != *tc.expectedCors.AllowCredentials {
					t.Errorf("Expected allowCredentials %v, got %v", *tc.expectedCors.AllowCredentials, cors.AllowCredentials)
				}
			} else if cors.AllowCredentials != nil {
				t.Errorf("Expected nil allowCredentials, got %v", *cors.AllowCredentials)
			}

			// Validate MaxAge
			if cors.MaxAge != tc.expectedCors.MaxAge {
				t.Errorf("Expected MaxAge %v, got %v", tc.expectedCors.MaxAge, cors.MaxAge)
			}
		})
	}
}

func TestCorsMaxAgeParsing(t *testing.T) {
	testCases := []struct {
		name          string
		annotationVal string
		expectedValid bool
		expectedVal   int32
	}{
		{
			name:          "valid integer",
			annotationVal: "100",
			expectedValid: true,
			expectedVal:   100,
		},
		{
			name:          "invalid value",
			annotationVal: "invalid",
			expectedValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pIR := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}
			eIR := emitterir.EmitterIR{
				HTTPRoutes: make(map[types.NamespacedName]emitterir.HTTPRouteContext),
			}

			ing := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cors-test",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/enable-cors":  "true",
						"nginx.ingress.kubernetes.io/cors-max-age": tc.annotationVal,
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			}

			key := types.NamespacedName{Namespace: ing.Namespace, Name: common.RouteName(ing.Name, "example.com")}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: ing.Namespace, Name: key.Name},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: ptr.To(gatewayv1.PathMatchPathPrefix), Value: ptr.To("/")}}},
						},
					},
				},
			}

			pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
				HTTPRoute: route,
				RuleBackendSources: [][]providerir.BackendSource{{
					{Ingress: &ing},
				}},
			}
			eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{
				HTTPRoute: route,
			}

			errs := applyCorsToEmitterIR(pIR, &eIR)

			if tc.expectedValid {
				if len(errs) > 0 {
					t.Fatalf("Unexpected errors: %v", errs)
				}
				if eIR.HTTPRoutes[key].CorsPolicyByRuleIdx == nil || eIR.HTTPRoutes[key].CorsPolicyByRuleIdx[0] == nil {
					t.Fatalf("Expected CORS policy, got nil")
				}
				cors := eIR.HTTPRoutes[key].CorsPolicyByRuleIdx[0]
				if cors.MaxAge != tc.expectedVal {
					t.Errorf("Expected MaxAge %d, got %d", tc.expectedVal, cors.MaxAge)
				}
			} else {
				if len(errs) == 0 {
					t.Fatal("Expected error for invalid value, got none")
				}
			}
		})
	}
}
