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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestCreateAppRootRedirectRoute(t *testing.T) {
	baseRoute := gatewayv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.networking.k8s.io/v1",
			Kind:       "HTTPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "default",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name: "my-gateway",
					},
				},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
		},
	}

	tests := []struct {
		name            string
		appRoot         string
		expectedPath    string
		expectedStatus  int
	}{
		{
			name:           "simple app-root",
			appRoot:        "/dashboard",
			expectedPath:   "/dashboard",
			expectedStatus: 302,
		},
		{
			name:           "nested app-root",
			appRoot:        "/app/v1/home",
			expectedPath:   "/app/v1/home",
			expectedStatus: 302,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := createAppRootRedirectRoute(baseRoute, tt.appRoot)

			// Check route name
			expectedName := "test-route-app-root"
			if route.Name != expectedName {
				t.Errorf("Route name = %q, expected %q", route.Name, expectedName)
			}

			// Check namespace
			if route.Namespace != baseRoute.Namespace {
				t.Errorf("Route namespace = %q, expected %q", route.Namespace, baseRoute.Namespace)
			}

			// Check hostnames are copied
			if len(route.Spec.Hostnames) != len(baseRoute.Spec.Hostnames) {
				t.Errorf("Route hostnames length = %d, expected %d", len(route.Spec.Hostnames), len(baseRoute.Spec.Hostnames))
			}

			// Check parent refs are copied
			if len(route.Spec.ParentRefs) != len(baseRoute.Spec.ParentRefs) {
				t.Errorf("Route parentRefs length = %d, expected %d", len(route.Spec.ParentRefs), len(baseRoute.Spec.ParentRefs))
			}

			// Check rules
			if len(route.Spec.Rules) != 1 {
				t.Fatalf("Expected 1 rule, got %d", len(route.Spec.Rules))
			}

			rule := route.Spec.Rules[0]

			// Check matches
			if len(rule.Matches) != 1 {
				t.Fatalf("Expected 1 match, got %d", len(rule.Matches))
			}

			match := rule.Matches[0]
			if match.Path == nil || match.Path.Value == nil || *match.Path.Value != "/" {
				t.Errorf("Expected path match '/', got %v", match.Path)
			}

			exactMatch := gatewayv1.PathMatchExact
			if match.Path.Type == nil || *match.Path.Type != exactMatch {
				t.Errorf("Expected exact path match, got %v", match.Path.Type)
			}

			// Check filters
			if len(rule.Filters) != 1 {
				t.Fatalf("Expected 1 filter, got %d", len(rule.Filters))
			}

			filter := rule.Filters[0]
			if filter.Type != gatewayv1.HTTPRouteFilterRequestRedirect {
				t.Errorf("Expected RequestRedirect filter, got %v", filter.Type)
			}

			if filter.RequestRedirect == nil {
				t.Fatal("RequestRedirect is nil")
			}

			if filter.RequestRedirect.Path == nil {
				t.Fatal("RequestRedirect.Path is nil")
			}

			if filter.RequestRedirect.Path.Type != gatewayv1.FullPathHTTPPathModifier {
				t.Errorf("Expected FullPathHTTPPathModifier, got %v", filter.RequestRedirect.Path.Type)
			}

			if filter.RequestRedirect.Path.ReplaceFullPath == nil || *filter.RequestRedirect.Path.ReplaceFullPath != tt.expectedPath {
				t.Errorf("Expected ReplaceFullPath = %q, got %v", tt.expectedPath, filter.RequestRedirect.Path.ReplaceFullPath)
			}

			if filter.RequestRedirect.StatusCode == nil || *filter.RequestRedirect.StatusCode != tt.expectedStatus {
				t.Errorf("Expected StatusCode = %d, got %v", tt.expectedStatus, filter.RequestRedirect.StatusCode)
			}
		})
	}
}

func TestApplyAppRootToEmitterIR(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	key := types.NamespacedName{Namespace: "default", Name: "test-route"}

	tests := []struct {
		name                 string
		ingress              networkingv1.Ingress
		expectRoute          bool
		expectedRouteName    string
		expectedAppRoot      string
		expectError          bool
	}{
		{
			name: "ingress with app-root annotation",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						AppRootAnnotation: "/dashboard",
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
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "my-service",
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
			expectRoute:       true,
			expectedRouteName: "test-route-app-root",
			expectedAppRoot:   "/dashboard",
			expectError:       false,
		},
		{
			name: "ingress without app-root annotation",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-ingress",
					Namespace:   "default",
					Annotations: map[string]string{},
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
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "my-service",
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
			expectRoute: false,
			expectError: false,
		},
		{
			name: "ingress with invalid app-root (no leading slash)",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						AppRootAnnotation: "dashboard",
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
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "my-service",
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
			expectRoute: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseRoute := gatewayv1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "gateway.networking.k8s.io/v1",
					Kind:       "HTTPRoute",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					CommonRouteSpec: gatewayv1.CommonRouteSpec{
						ParentRefs: []gatewayv1.ParentReference{
							{Name: "my-gateway"},
						},
					},
					Hostnames: []gatewayv1.Hostname{"example.com"},
				},
			}

			pIR := providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					key: {
						HTTPRoute: baseRoute,
						RuleBackendSources: [][]providerir.BackendSource{
							{{Ingress: &tt.ingress}},
						},
					},
				},
			}

			eIR := emitterir.EmitterIR{
				HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
					key: {HTTPRoute: baseRoute},
				},
			}

			errs := applyAppRootToEmitterIR(pIR, &eIR)

			if tt.expectError {
				if len(errs) == 0 {
					t.Error("Expected error but got none")
				}
				return
			}

			if len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
			}

			appRootKey := types.NamespacedName{Namespace: key.Namespace, Name: tt.expectedRouteName}

			if tt.expectRoute {
				route, ok := eIR.HTTPRoutes[appRootKey]
				if !ok {
					t.Fatalf("Expected app-root route %v to be created", appRootKey)
				}

				// Verify the redirect path
				if len(route.Spec.Rules) != 1 || len(route.Spec.Rules[0].Filters) != 1 {
					t.Fatal("Expected 1 rule with 1 filter")
				}

				filter := route.Spec.Rules[0].Filters[0]
				if filter.RequestRedirect == nil || filter.RequestRedirect.Path == nil {
					t.Fatal("Expected RequestRedirect with Path")
				}

				if *filter.RequestRedirect.Path.ReplaceFullPath != tt.expectedAppRoot {
					t.Errorf("Expected app-root = %q, got %q", tt.expectedAppRoot, *filter.RequestRedirect.Path.ReplaceFullPath)
				}
			} else {
				if _, ok := eIR.HTTPRoutes[appRootKey]; ok {
					t.Errorf("Did not expect app-root route to be created")
				}
			}
		})
	}
}

func TestApplyAppRootToEmitterIR_PreservesParentRefs(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "test-route"}
	pathType := networkingv1.PathTypePrefix

	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				AppRootAnnotation: "/home",
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
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-service",
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

	parentRefs := []gatewayv1.ParentReference{
		{
			Name:      "gateway-1",
			Namespace: ptr.To(gatewayv1.Namespace("gateway-ns")),
		},
		{
			Name:      "gateway-2",
			Namespace: ptr.To(gatewayv1.Namespace("gateway-ns")),
		},
	}

	baseRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
		},
	}

	pIR := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				HTTPRoute: baseRoute,
				RuleBackendSources: [][]providerir.BackendSource{
					{{Ingress: &ingress}},
				},
			},
		},
	}

	eIR := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {HTTPRoute: baseRoute},
		},
	}

	errs := applyAppRootToEmitterIR(pIR, &eIR)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	appRootKey := types.NamespacedName{Namespace: key.Namespace, Name: "test-route-app-root"}
	route, ok := eIR.HTTPRoutes[appRootKey]
	if !ok {
		t.Fatalf("Expected app-root route to be created")
	}

	if len(route.Spec.ParentRefs) != len(parentRefs) {
		t.Errorf("Expected %d parentRefs, got %d", len(parentRefs), len(route.Spec.ParentRefs))
	}

	for i, ref := range route.Spec.ParentRefs {
		if ref.Name != parentRefs[i].Name {
			t.Errorf("ParentRef[%d] name = %q, expected %q", i, ref.Name, parentRefs[i].Name)
		}
	}
}
