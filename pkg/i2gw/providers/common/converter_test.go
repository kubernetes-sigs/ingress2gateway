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

package common

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_ToIR(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	iExact := networkingv1.PathTypeExact
	gPathPrefix := gatewayv1.PathMatchPathPrefix
	gExact := gatewayv1.PathMatchExact

	testCases := []struct {
		name           string
		ingresses      []networkingv1.Ingress
		expectedIR     intermediate.IR
		expectedErrors field.ErrorList
	}{
		{
			name:           "empty",
			ingresses:      []networkingv1.Ingress{},
			expectedIR:     intermediate.IR{},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "simple ingress",
			ingresses: []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "simple", Namespace: "test"},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/foo",
									PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "example",
											Port: networkingv1.ServiceBackendPort{
												Number: 3000,
											},
										},
									},
								}},
							},
						},
					}},
					IngressClassName: PtrTo("simple"),
				},
			}},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: "test", Name: "simple"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "simple", Namespace: "test"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "simple",
								Listeners: []gatewayv1.Listener{{
									Name:     "example-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: PtrTo(gatewayv1.Hostname("example.com")),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: "test", Name: "simple-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "simple-example-com", Namespace: "test"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "simple",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"example.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gPathPrefix,
											Value: PtrTo("/foo"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "example",
												Port: PtrTo(gatewayv1.PortNumber(3000)),
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
			name: "ingress with TLS",
			ingresses: []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "with-tls", Namespace: "test"},
				Spec: networkingv1.IngressSpec{
					TLS: []networkingv1.IngressTLS{{
						Hosts:      []string{"example.com"},
						SecretName: "example-cert",
					}},
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/foo",
									PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "example",
											Port: networkingv1.ServiceBackendPort{
												Number: 3000,
											},
										},
									},
								}},
							},
						},
					}},
					IngressClassName: PtrTo("with-tls"),
				},
			}},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: "test", Name: "with-tls"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "with-tls", Namespace: "test"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "with-tls",
								Listeners: []gatewayv1.Listener{{
									Name:     "example-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: PtrTo(gatewayv1.Hostname("example.com")),
								}, {
									Name:     "example-com-https",
									Port:     443,
									Protocol: gatewayv1.HTTPSProtocolType,
									Hostname: PtrTo(gatewayv1.Hostname("example.com")),
									TLS: &gatewayv1.GatewayTLSConfig{
										CertificateRefs: []gatewayv1.SecretObjectReference{{
											Name: "example-cert",
										}},
									},
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: "test", Name: "with-tls-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "with-tls-example-com", Namespace: "test"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "with-tls",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"example.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gPathPrefix,
											Value: PtrTo("/foo"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "example",
												Port: PtrTo(gatewayv1.PortNumber(3000)),
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
			name: "ingress with custom and default backend",
			ingresses: []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "net", Namespace: "different"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: PtrTo("example-proxy"),
					Rules: []networkingv1.IngressRule{{
						Host: "example.net",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/bar",
									PathType: &iExact,
									Backend: networkingv1.IngressBackend{
										Resource: &corev1.TypedLocalObjectReference{
											Name:     "custom",
											Kind:     "StorageBucket",
											APIGroup: PtrTo("vendor.example.com"),
										},
									},
								}},
							},
						},
					}},
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "default",
							Port: networkingv1.ServiceBackendPort{
								Number: 8080,
							},
						},
					},
				},
			}},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: "different", Name: "example-proxy"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "example-proxy", Namespace: "different"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "example-proxy",
								Listeners: []gatewayv1.Listener{{
									Name:     "example-net-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: PtrTo(gatewayv1.Hostname("example.net")),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: "different", Name: "net-example-net"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "net-example-net", Namespace: "different"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "example-proxy",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"example.net"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gExact,
											Value: PtrTo("/bar"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name:  "custom",
												Group: PtrTo(gatewayv1.Group("vendor.example.com")),
												Kind:  PtrTo(gatewayv1.Kind("StorageBucket")),
											},
										},
									}},
								}},
							},
						},
					},
					{Namespace: "different", Name: "net-default-backend"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "net-default-backend", Namespace: "different"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "example-proxy",
									}},
								},
								Rules: []gatewayv1.HTTPRouteRule{{
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "default",
												Port: PtrTo(gatewayv1.PortNumber(8080)),
											},
										}},
									}},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "duplicated backends",
			ingresses: []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "duplicate-a", Namespace: "test"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: PtrTo("example-proxy"),
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/foo",
									PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "example",
											Port: networkingv1.ServiceBackendPort{
												Number: 3000,
											},
										},
									},
								}},
							},
						},
					}},
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{Name: "duplicate-b", Namespace: "test"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: PtrTo("example-proxy"),
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/foo",
									PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "example",
											Port: networkingv1.ServiceBackendPort{
												Number: 3000,
											},
										},
									},
								}},
							},
						},
					}},
				},
			}},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: "test", Name: "example-proxy"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "example-proxy", Namespace: "test"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "example-proxy",
								Listeners: []gatewayv1.Listener{{
									Name:     "example-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: PtrTo(gatewayv1.Hostname("example.com")),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: "test", Name: "duplicate-a-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "duplicate-a-example-com", Namespace: "test"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "example-proxy",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"example.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  &gPathPrefix,
											Value: PtrTo("/foo"),
										},
									}},
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "example",
												Port: PtrTo(gatewayv1.PortNumber(3000)),
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			ir, errs := ToIR(tc.ingresses, i2gw.ProviderImplementationSpecificOptions{}, noNotifications)
			if len(ir.HTTPRoutes) != len(tc.expectedIR.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedIR.HTTPRoutes), len(ir.HTTPRoutes), ir.HTTPRoutes)
			} else {
				for i, gotHTTPRouteContext := range ir.HTTPRoutes {
					key := types.NamespacedName{Namespace: gotHTTPRouteContext.HTTPRoute.Namespace, Name: gotHTTPRouteContext.HTTPRoute.Name}
					wantHTTPRouteContext := tc.expectedIR.HTTPRoutes[key]
					wantHTTPRouteContext.HTTPRoute.SetGroupVersionKind(HTTPRouteGVK)
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
					wantGatewayContext.Gateway.SetGroupVersionKind(GatewayGVK)
					if !apiequality.Semantic.DeepEqual(gotGatewayContext.Gateway, wantGatewayContext.Gateway) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, wantGatewayContext.Gateway, gotGatewayContext.Gateway, cmp.Diff(wantGatewayContext.Gateway, gotGatewayContext.Gateway))
					}
				}
			}

			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
			}
		})
	}
}
