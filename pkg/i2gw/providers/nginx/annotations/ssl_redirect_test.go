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

func TestSSLRedirectFeature(t *testing.T) {
	tests := []struct {
		name           string
		annotations    map[string]string
		expectRedirect bool
	}{
		{
			name: "modern NGINX redirect annotation",
			annotations: map[string]string{
				nginxRedirectToHTTPSAnnotation: "true",
			},
			expectRedirect: true,
		},
		{
			name: "legacy SSL redirect annotation",
			annotations: map[string]string{
				legacySSLRedirectAnnotation: "true",
			},
			expectRedirect: true,
		},
		{
			name:           "no annotations",
			annotations:    map[string]string{},
			expectRedirect: false,
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

			// Setup IR with existing Gateway and HTTPRoute
			routeName := common.RouteName(ingress.Name, ingress.Spec.Rules[0].Host)
			routeKey := types.NamespacedName{Namespace: ingress.Namespace, Name: routeName}
			gatewayKey := types.NamespacedName{Namespace: ingress.Namespace, Name: "nginx"}

			ir := providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					gatewayKey: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "nginx",
								Namespace: "default",
							},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "nginx",
								Listeners: []gatewayv1.Listener{
									{
										Name:     "http",
										Port:     80,
										Protocol: gatewayv1.HTTPProtocolType,
									},
								},
							},
						},
					},
				},
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
			errs := SSLRedirectFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
			if len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
				return
			}

			// Verify results
			if !tt.expectRedirect {
				// Should not have added HTTPS listener
				gateway := ir.Gateways[gatewayKey].Gateway
				httpsListeners := 0
				for _, listener := range gateway.Spec.Listeners {
					if listener.Protocol == gatewayv1.HTTPSProtocolType {
						httpsListeners++
					}
				}
				if httpsListeners > 0 {
					t.Errorf("Expected no HTTPS listeners, got %d", httpsListeners)
				}
				return
			}

			// Verify HTTPS listener was added
			gateway := ir.Gateways[gatewayKey].Gateway
			httpsListenerFound := false
			for _, listener := range gateway.Spec.Listeners {
				if listener.Protocol == gatewayv1.HTTPSProtocolType {
					httpsListenerFound = true
					break
				}
			}
			if !httpsListenerFound {
				t.Error("Expected HTTPS listener to be added")
			}

			// Verify HTTPRoute modifications
			httpRoute := ir.HTTPRoutes[routeKey].HTTPRoute

			// Verify parentRefs sectionName is set
			if len(httpRoute.Spec.ParentRefs) == 0 || httpRoute.Spec.ParentRefs[0].SectionName == nil {
				t.Error("Expected parentRefs sectionName to be set")
			}

			// Verify redirect rule was added
			if len(httpRoute.Spec.Rules) < 2 {
				t.Errorf("Expected at least 2 rules (redirect + original)")
				return
			}

			// First rule should be the redirect rule
			redirectRule := httpRoute.Spec.Rules[0]
			if len(redirectRule.Filters) == 0 || redirectRule.Filters[0].Type != gatewayv1.HTTPRouteFilterRequestRedirect {
				t.Error("Expected RequestRedirect filter in first rule")
			}

			// Verify redirect filter configuration
			if redirectRule.Filters[0].RequestRedirect == nil {
				t.Error("Expected RequestRedirect to be configured")
			} else {
				if redirectRule.Filters[0].RequestRedirect.Scheme == nil || *redirectRule.Filters[0].RequestRedirect.Scheme != "https" {
					t.Error("Expected redirect scheme to be 'https'")
				}
				if redirectRule.Filters[0].RequestRedirect.StatusCode == nil || *redirectRule.Filters[0].RequestRedirect.StatusCode != 301 {
					t.Error("Expected redirect status code to be 301")
				}
			}
		})
	}
}
