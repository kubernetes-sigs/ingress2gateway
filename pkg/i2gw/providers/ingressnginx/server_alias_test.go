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

package ingressnginx

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestServerAliasFeature(t *testing.T) {
	testCases := []struct {
		name           string
		ingresses      []networkingv1.Ingress
		initialIR      intermediate.IR
		expectedIR     intermediate.IR
		expectedErrors int
	}{
		{
			name: "ingress with server alias annotation",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/server-alias": "api.example.com,cdn.example.com",
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("test-gateway"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: ptrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "test-service",
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
			},
			initialIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Name: "test-gateway", Namespace: "default"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-gateway",
								Namespace: "default",
							},
							Spec: gatewayv1.GatewaySpec{
								Listeners: []gatewayv1.Listener{
									{
										Name:     "http",
										Protocol: gatewayv1.HTTPProtocolType,
										Port:     80,
										Hostname: ptrTo(gatewayv1.Hostname("example.com")),
									},
								},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Name: "test-route", Namespace: "default"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-route",
								Namespace: "default",
							},
							Spec: gatewayv1.HTTPRouteSpec{
								Hostnames: []gatewayv1.Hostname{"example.com"},
							},
						},
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Name: "test-gateway", Namespace: "default"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-gateway",
								Namespace: "default",
							},
							Spec: gatewayv1.GatewaySpec{
								Listeners: []gatewayv1.Listener{
									{
										Name:     "http",
										Protocol: gatewayv1.HTTPProtocolType,
										Port:     80,
										Hostname: ptrTo(gatewayv1.Hostname("example.com")),
									},
								},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Name: "test-route", Namespace: "default"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-route",
								Namespace: "default",
							},
							Spec: gatewayv1.HTTPRouteSpec{
								Hostnames: []gatewayv1.Hostname{
									"example.com",
									"api.example.com",
									"cdn.example.com",
								},
							},
						},
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "ingress without server alias annotation",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "example.com",
							},
						},
					},
				},
			},
			initialIR: intermediate.IR{
				Gateways:   map[types.NamespacedName]intermediate.GatewayContext{},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{},
			},
			expectedIR: intermediate.IR{
				Gateways:   map[types.NamespacedName]intermediate.GatewayContext{},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{},
			},
			expectedErrors: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := tc.initialIR
			errs := serverAliasFeature(tc.ingresses, nil, &ir)

			if len(errs) != tc.expectedErrors {
				t.Fatalf("Expected %d errors, got %d: %v", tc.expectedErrors, len(errs), errs)
			}

			// Compare gateways
			if diff := cmp.Diff(tc.expectedIR.Gateways, ir.Gateways); diff != "" {
				t.Fatalf("Gateways mismatch (-want +got):\n%s", diff)
			}

			// Compare HTTPRoutes
			if diff := cmp.Diff(tc.expectedIR.HTTPRoutes, ir.HTTPRoutes); diff != "" {
				t.Fatalf("HTTPRoutes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
