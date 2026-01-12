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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_ToIR(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	// iExact := networkingv1.PathTypeExact
	isPathType := networkingv1.PathTypeImplementationSpecific
	gPathPrefix := gatewayv1.PathMatchPathPrefix
	// gExact := gatewayv1.PathMatchExact

	testCases := []struct {
		name           string
		ingresses      OrderedIngressMap
		expectedIR     providerir.ProviderIR
		expectedErrors field.ErrorList
	}{
		{
			name: "canary deployment",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "production"}, {Namespace: "default", Name: "canary"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "production"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "production", Namespace: "default"},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("ingress-nginx"),
							Rules: []networkingv1.IngressRule{{
								Host: "echo.prod.mydomain.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Resource: &apiv1.TypedLocalObjectReference{
													Name:     "production",
													Kind:     "StorageBucket",
													APIGroup: ptrTo("vendor.example.com"),
												},
											},
										}},
									},
								},
							}},
						},
					},
					{Namespace: "default", Name: "canary"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "canary",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/canary":        "true",
								"nginx.ingress.kubernetes.io/canary-weight": "20",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("ingress-nginx"),
							Rules: []networkingv1.IngressRule{{
								Host: "echo.prod.mydomain.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Resource: &apiv1.TypedLocalObjectReference{
													Name:     "canary",
													Kind:     "StorageBucket",
													APIGroup: ptrTo("vendor.example.com"),
												},
											},
										}},
									},
								},
							}},
						},
					},
				},
			},
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "ingress-nginx"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "ingress-nginx", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "ingress-nginx",
								Listeners: []gatewayv1.Listener{{
									Name:     "echo-prod-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname("echo.prod.mydomain.com")),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "production-echo-prod-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "production-echo-prod-mydomain-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "ingress-nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"echo.prod.mydomain.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gPathPrefix,
											Value: ptrTo("/"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name:  "production",
													Group: ptrTo(gatewayv1.Group("vendor.example.com")),
													Kind:  ptrTo(gatewayv1.Kind("StorageBucket")),
												},
												Weight: ptrTo(int32(80)),
											},
										},
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name:  "canary",
													Group: ptrTo(gatewayv1.Group("vendor.example.com")),
													Kind:  ptrTo(gatewayv1.Kind("StorageBucket")),
												},
												Weight: ptrTo(int32(20)),
											},
										},
									},
								}},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "canary deployment total weight",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "production"}, {Namespace: "default", Name: "canary"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "production"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "production", Namespace: "default"},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("ingress-nginx"),
							Rules: []networkingv1.IngressRule{{
								Host: "echo.prod.mydomain.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Resource: &apiv1.TypedLocalObjectReference{
													Name:     "production",
													Kind:     "StorageBucket",
													APIGroup: ptrTo("vendor.example.com"),
												},
											},
										}},
									},
								},
							}},
						},
					},
					{Namespace: "default", Name: "canary"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "canary",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/canary":              "true",
								"nginx.ingress.kubernetes.io/canary-weight":       "20",
								"nginx.ingress.kubernetes.io/canary-weight-total": "200",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("ingress-nginx"),
							Rules: []networkingv1.IngressRule{{
								Host: "echo.prod.mydomain.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Resource: &apiv1.TypedLocalObjectReference{
													Name:     "canary",
													Kind:     "StorageBucket",
													APIGroup: ptrTo("vendor.example.com"),
												},
											},
										}},
									},
								},
							}},
						},
					},
				},
			},
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "ingress-nginx"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "ingress-nginx", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "ingress-nginx",
								Listeners: []gatewayv1.Listener{{
									Name:     "echo-prod-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname("echo.prod.mydomain.com")),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "production-echo-prod-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "production-echo-prod-mydomain-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "ingress-nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"echo.prod.mydomain.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gPathPrefix,
											Value: ptrTo("/"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name:  "production",
													Group: ptrTo(gatewayv1.Group("vendor.example.com")),
													Kind:  ptrTo(gatewayv1.Kind("StorageBucket")),
												},
												Weight: ptrTo(int32(180)),
											},
										},
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name:  "canary",
													Group: ptrTo(gatewayv1.Group("vendor.example.com")),
													Kind:  ptrTo(gatewayv1.Kind("StorageBucket")),
												},
												Weight: ptrTo(int32(20)),
											},
										},
									},
								}},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ImplementationSpecific HTTPRouteMatching",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "implementation-specific-regex"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "implementation-specific-regex"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "implementation-specific-regex",
							Namespace: "default",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("ingress-nginx"),
							Rules: []networkingv1.IngressRule{{
								Host: "test.mydomain.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/~/echo/**/test",
											PathType: &isPathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "test",
													Port: networkingv1.ServiceBackendPort{
														Number: 80,
													},
												},
											},
										}},
									},
								},
							}},
						},
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: "default", Name: "ingress-nginx"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "ingress-nginx", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "ingress-nginx",
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname("test.mydomain.com")),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: "default", Name: "implementation-specific-regex-test-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "implementation-specific-regex-test-mydomain-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "ingress-nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"test.mydomain.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  ptrTo(gatewayv1.PathMatchRegularExpression),
											Value: ptrTo("/~/echo/**/test"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "test",
												Port: ptrTo(gatewayv1.PortNumber(80)),
											},
										},
									}},
								}},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "multiple rules with TLS",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "example-ingress"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "example-ingress"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "example-ingress", Namespace: "default"},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("nginx"),
							TLS: []networkingv1.IngressTLS{{
								Hosts: []string{
									"foo.example.com",
									"bar.example.com",
								},
								SecretName: "example-com",
							}},
							Rules: []networkingv1.IngressRule{
								{
									Host: "foo.example.com",
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: &iPrefix,
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "foo-app",
															Port: networkingv1.ServiceBackendPort{Number: 80},
														},
													},
												},
												{
													Path:     "/orders",
													PathType: &iPrefix,
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "foo-orders-app",
															Port: networkingv1.ServiceBackendPort{Number: 80},
														},
													},
												},
											},
										},
									},
								},
								{
									Host: "bar.example.com",
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: &iPrefix,
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "bar-app",
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
			},
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "nginx"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "nginx",
								Listeners: []gatewayv1.Listener{
									{
										Name:     "bar-example-com-http",
										Port:     80,
										Protocol: gatewayv1.HTTPProtocolType,
										Hostname: ptrTo(gatewayv1.Hostname("bar.example.com")),
									},
									{
										Name:     "bar-example-com-https",
										Port:     443,
										Protocol: gatewayv1.HTTPSProtocolType,
										Hostname: ptrTo(gatewayv1.Hostname("bar.example.com")),
										TLS: &gatewayv1.ListenerTLSConfig{
											CertificateRefs: []gatewayv1.SecretObjectReference{
												{Name: "example-com"},
											},
										},
									},
									{
										Name:     "foo-example-com-http",
										Port:     80,
										Protocol: gatewayv1.HTTPProtocolType,
										Hostname: ptrTo(gatewayv1.Hostname("foo.example.com")),
									},
									{
										Name:     "foo-example-com-https",
										Port:     443,
										Protocol: gatewayv1.HTTPSProtocolType,
										Hostname: ptrTo(gatewayv1.Hostname("foo.example.com")),
										TLS: &gatewayv1.ListenerTLSConfig{
											CertificateRefs: []gatewayv1.SecretObjectReference{
												{Name: "example-com"},
											},
										},
									},
								},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "example-ingress-bar-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "example-ingress-bar-example-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"bar.example.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gPathPrefix,
											Value: ptrTo("/"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "bar-app",
													Port: ptrTo(gatewayv1.PortNumber(80)),
												},
											},
										},
									},
								}},
							},
						},
					},
					{Namespace: "default", Name: "example-ingress-foo-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "example-ingress-foo-example-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"foo.example.com"},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
										}},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "foo-app",
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
									{
										Matches: []gatewayv1.HTTPRouteMatch{{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/orders"),
											},
										}},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "foo-orders-app",
														Port: ptrTo(gatewayv1.PortNumber(80)),
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
			expectedErrors: field.ErrorList{},
		},
		{
			name: "multiple rules with canary",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{
					{Namespace: "default", Name: "example-ingress"},
					{Namespace: "default", Name: "example-ingress-canary"},
				},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "example-ingress"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "example-ingress", Namespace: "default"},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("nginx"),
							Rules: []networkingv1.IngressRule{
								{
									Host: "foo.example.com",
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: &iPrefix,
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "foo-app",
															Port: networkingv1.ServiceBackendPort{Number: 80},
														},
													},
												},
												{
													Path:     "/orders",
													PathType: &iPrefix,
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "foo-orders-app",
															Port: networkingv1.ServiceBackendPort{Number: 80},
														},
													},
												},
											},
										},
									},
								},
								{
									Host: "bar.example.com",
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: &iPrefix,
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "bar-app",
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
					{Namespace: "default", Name: "example-ingress-canary"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-ingress-canary",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/canary":        "true",
								"nginx.ingress.kubernetes.io/canary-weight": "30",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("nginx"),
							Rules: []networkingv1.IngressRule{
								{
									Host: "bar.example.com",
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: &iPrefix,
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "bar-app-canary",
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
			},
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "nginx"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "nginx",
								Listeners: []gatewayv1.Listener{
									{
										Name:     "bar-example-com-http",
										Port:     80,
										Protocol: gatewayv1.HTTPProtocolType,
										Hostname: ptrTo(gatewayv1.Hostname("bar.example.com")),
									},
									{
										Name:     "foo-example-com-http",
										Port:     80,
										Protocol: gatewayv1.HTTPProtocolType,
										Hostname: ptrTo(gatewayv1.Hostname("foo.example.com")),
									},
								},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "example-ingress-bar-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "example-ingress-bar-example-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"bar.example.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gPathPrefix,
											Value: ptrTo("/"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "bar-app",
													Port: ptrTo(gatewayv1.PortNumber(80)),
												},
												Weight: ptrTo[int32](70),
											},
										},
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "bar-app-canary",
													Port: ptrTo(gatewayv1.PortNumber(80)),
												},
												Weight: ptrTo[int32](30),
											},
										},
									},
								}},
							},
						},
					},
					{Namespace: "default", Name: "example-ingress-foo-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "example-ingress-foo-example-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"foo.example.com"},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
										}},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "foo-app",
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
									{
										Matches: []gatewayv1.HTTPRouteMatch{{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/orders"),
											},
										}},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "foo-orders-app",
														Port: ptrTo(gatewayv1.PortNumber(80)),
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
			expectedErrors: field.ErrorList{},
		},
		{
			name: "canary with same service in different rules - based on bad.yaml",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "production-ingress"}, {Namespace: "default", Name: "canary-ingress"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "production-ingress"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "production-ingress", Namespace: "default"},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("nginx"),
							Rules: []networkingv1.IngressRule{{
								Host: "api.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/api",
												PathType: &iPrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "api-service-v1",
														Port: networkingv1.ServiceBackendPort{Number: 80},
													},
												},
											},
											{
												Path:     "/admin",
												PathType: &iPrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "admin-service",
														Port: networkingv1.ServiceBackendPort{Number: 80},
													},
												},
											},
										},
									},
								},
							}},
						},
					},
					{Namespace: "default", Name: "canary-ingress"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "canary-ingress",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/canary":        "true",
								"nginx.ingress.kubernetes.io/canary-weight": "10",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptrTo("nginx"),
							Rules: []networkingv1.IngressRule{{
								Host: "api.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/api",
												PathType: &iPrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "api-service-v2",
														Port: networkingv1.ServiceBackendPort{Number: 80},
													},
												},
											},
											{
												Path:     "/admin",
												PathType: &iPrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "api-service-v1", // Same service as production's "/api" path!
														Port: networkingv1.ServiceBackendPort{Number: 80},
													},
												},
											},
										},
									},
								},
							}},
						},
					},
				},
			},
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "nginx"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "nginx",
								Listeners: []gatewayv1.Listener{{
									Name:     "api-example-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname("api.example.com")),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "production-ingress-api-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "production-ingress-api-example-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "nginx",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"api.example.com"},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/api"),
											},
										}},
										// Path "/api" has api-service-v1 from production and api-service-v2 from canary
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "api-service-v1",
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
													Weight: ptrTo(int32(90)), // Production gets 90%
												},
											},
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "api-service-v2",
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
													Weight: ptrTo(int32(10)), // Canary gets 10%
												},
											},
										},
									},
									{
										Matches: []gatewayv1.HTTPRouteMatch{{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/admin"),
											},
										}},
										// Path "/admin" has admin-service from production and api-service-v1 from canary
										// Note: api-service-v1 appears in both rules but with different weights based on source!
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "admin-service",
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
													Weight: ptrTo(int32(90)), // Production gets 90%
												},
											},
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "api-service-v1",
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
													Weight: ptrTo(int32(10)), // Canary gets 10%
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
			expectedErrors: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewProvider(&i2gw.ProviderConf{})

			nginxProvider := provider.(*Provider)
			nginxProvider.storage.Ingresses = tc.ingresses

			ir, errs := provider.ToIR()

			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
			}

			if len(ir.HTTPRoutes) != len(tc.expectedIR.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedIR.HTTPRoutes), len(ir.HTTPRoutes), ir.HTTPRoutes)
			} else {
				for i, gotHTTPRouteContext := range ir.HTTPRoutes {
					key := types.NamespacedName{Namespace: gotHTTPRouteContext.HTTPRoute.Namespace, Name: gotHTTPRouteContext.HTTPRoute.Name}
					wantHTTPRouteContext := tc.expectedIR.HTTPRoutes[key]
					wantHTTPRouteContext.HTTPRoute.SetGroupVersionKind(common.HTTPRouteGVK)
					if !apiequality.Semantic.DeepEqual(gotHTTPRouteContext.HTTPRoute, wantHTTPRouteContext.HTTPRoute) {
						t.Errorf("Expected HTTPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, wantHTTPRouteContext.HTTPRoute, gotHTTPRouteContext.HTTPRoute, cmp.Diff(wantHTTPRouteContext.HTTPRoute, gotHTTPRouteContext.HTTPRoute))
					}
				}
			}

			if len(ir.Gateways) != len(tc.expectedIR.Gateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v",
					len(tc.expectedIR.Gateways), len(ir.Gateways), ir.Gateways)
			} else {
				for i, gotGatewayContext := range ir.Gateways {
					key := types.NamespacedName{Namespace: gotGatewayContext.Gateway.Namespace, Name: gotGatewayContext.Gateway.Name}
					wantGatewayContext := tc.expectedIR.Gateways[key]
					wantGatewayContext.Gateway.SetGroupVersionKind(common.GatewayGVK)
					if !apiequality.Semantic.DeepEqual(gotGatewayContext.Gateway, wantGatewayContext.Gateway) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, wantGatewayContext.Gateway, gotGatewayContext.Gateway, cmp.Diff(wantGatewayContext.Gateway, gotGatewayContext.Gateway))
					}
				}
			}
		})
	}
}

func ptrTo[T any](a T) *T {
	return &a
}
