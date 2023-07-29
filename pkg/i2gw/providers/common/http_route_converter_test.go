/*
Copyright 2022 The Kubernetes Authors.

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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func Test_ingresses2GatewaysAndHttpRoutes(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	iExact := networkingv1.PathTypeExact
	gPathPrefix := gatewayv1beta1.PathMatchPathPrefix
	gExact := gatewayv1beta1.PathMatchExact

	testCases := []struct {
		name                     string
		ingresses                []networkingv1.Ingress
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
	}{{
		name:                     "empty",
		ingresses:                []networkingv1.Ingress{},
		expectedGatewayResources: i2gw.GatewayResources{},
		expectedErrors:           field.ErrorList{},
	},
		{
			name: "simple ingress",
			ingresses: []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test"},
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
				},
			}},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[i2gw.GatewayKey]gatewayv1beta1.Gateway{
					"test:example": {
						ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test"},
						Spec: gatewayv1beta1.GatewaySpec{
							GatewayClassName: "example",
							Listeners: []gatewayv1beta1.Listener{{
								Name:     "example-com-http",
								Port:     80,
								Protocol: gatewayv1beta1.HTTPProtocolType,
								Hostname: gatewayHostnamePtr("example.com"),
							}},
						},
					},
				},
				HTTPRoutes: map[i2gw.HTTPRouteKey]gatewayv1beta1.HTTPRoute{
					"test:example-com": {
						ObjectMeta: metav1.ObjectMeta{Name: "example-com", Namespace: "test"},
						Spec: gatewayv1beta1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
								ParentRefs: []gatewayv1beta1.ParentReference{{
									Name: "example",
								}},
							},
							Hostnames: []gatewayv1beta1.Hostname{"example.com"},
							Rules: []gatewayv1beta1.HTTPRouteRule{{
								Matches: []gatewayv1beta1.HTTPRouteMatch{{
									Path: &gatewayv1beta1.HTTPPathMatch{
										Type:  &gPathPrefix,
										Value: stringPtr("/foo"),
									},
								}},
								BackendRefs: []gatewayv1beta1.HTTPBackendRef{{
									BackendRef: gatewayv1beta1.BackendRef{
										BackendObjectReference: gatewayv1beta1.BackendObjectReference{
											Name: "example",
											Port: portNumberPtr(3000),
										},
									},
								}},
							}},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ingress with TLS",
			ingresses: []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test"},
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
				},
			}},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[i2gw.GatewayKey]gatewayv1beta1.Gateway{
					"test:example": {
						ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test"},
						Spec: gatewayv1beta1.GatewaySpec{
							GatewayClassName: "example",
							Listeners: []gatewayv1beta1.Listener{{
								Name:     "example-com-http",
								Port:     80,
								Protocol: gatewayv1beta1.HTTPProtocolType,
								Hostname: gatewayHostnamePtr("example.com"),
							}, {
								Name:     "example-com-https",
								Port:     443,
								Protocol: gatewayv1beta1.HTTPSProtocolType,
								Hostname: gatewayHostnamePtr("example.com"),
								TLS: &gatewayv1beta1.GatewayTLSConfig{
									CertificateRefs: []gatewayv1beta1.SecretObjectReference{{
										Name: "example-cert",
									}},
								},
							}},
						},
					},
				},
				HTTPRoutes: map[i2gw.HTTPRouteKey]gatewayv1beta1.HTTPRoute{
					"test:example-com": {
						ObjectMeta: metav1.ObjectMeta{Name: "example-com", Namespace: "test"},
						Spec: gatewayv1beta1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
								ParentRefs: []gatewayv1beta1.ParentReference{{
									Name: "example",
								}},
							},
							Hostnames: []gatewayv1beta1.Hostname{"example.com"},
							Rules: []gatewayv1beta1.HTTPRouteRule{{
								Matches: []gatewayv1beta1.HTTPRouteMatch{{
									Path: &gatewayv1beta1.HTTPPathMatch{
										Type:  &gPathPrefix,
										Value: stringPtr("/foo"),
									},
								}},
								BackendRefs: []gatewayv1beta1.HTTPBackendRef{{
									BackendRef: gatewayv1beta1.BackendRef{
										BackendObjectReference: gatewayv1beta1.BackendObjectReference{
											Name: "example",
											Port: portNumberPtr(3000),
										},
									},
								}},
							}},
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
					IngressClassName: stringPtr("example-proxy"),
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
											APIGroup: stringPtr("vendor.example.com"),
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[i2gw.GatewayKey]gatewayv1beta1.Gateway{
					"different:example-proxy": {
						ObjectMeta: metav1.ObjectMeta{Name: "example-proxy", Namespace: "different"},
						Spec: gatewayv1beta1.GatewaySpec{
							GatewayClassName: "example-proxy",
							Listeners: []gatewayv1beta1.Listener{{
								Name:     "example-net-http",
								Port:     80,
								Protocol: gatewayv1beta1.HTTPProtocolType,
								Hostname: gatewayHostnamePtr("example.net"),
							}},
						},
					},
				},
				HTTPRoutes: map[i2gw.HTTPRouteKey]gatewayv1beta1.HTTPRoute{
					"different:example-net": {
						ObjectMeta: metav1.ObjectMeta{Name: "example-net", Namespace: "different"},
						Spec: gatewayv1beta1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
								ParentRefs: []gatewayv1beta1.ParentReference{{
									Name: "example-proxy",
								}},
							},
							Hostnames: []gatewayv1beta1.Hostname{"example.net"},
							Rules: []gatewayv1beta1.HTTPRouteRule{{
								Matches: []gatewayv1beta1.HTTPRouteMatch{{
									Path: &gatewayv1beta1.HTTPPathMatch{
										Type:  &gExact,
										Value: stringPtr("/bar"),
									},
								}},
								BackendRefs: []gatewayv1beta1.HTTPBackendRef{{
									BackendRef: gatewayv1beta1.BackendRef{
										BackendObjectReference: gatewayv1beta1.BackendObjectReference{
											Name:  "custom",
											Group: apiGroupPtr("vendor.example.com"),
											Kind:  apiKindPtr("StorageBucket"),
										},
									},
								}},
							}},
						},
					},
					"different:net-default-backend": {
						ObjectMeta: metav1.ObjectMeta{Name: "net-default-backend", Namespace: "different"},
						Spec: gatewayv1beta1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
								ParentRefs: []gatewayv1beta1.ParentReference{{
									Name: "example-proxy",
								}},
							},
							Rules: []gatewayv1beta1.HTTPRouteRule{{
								BackendRefs: []gatewayv1beta1.HTTPBackendRef{{
									BackendRef: gatewayv1beta1.BackendRef{
										BackendObjectReference: gatewayv1beta1.BackendObjectReference{
											Name: "default",
											Port: portNumberPtr(8080),
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

			gatewayResources, errs := IngressToGateway(tc.ingresses)

			if len(gatewayResources.HTTPRoutes) != len(tc.expectedGatewayResources.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.HTTPRoutes), len(gatewayResources.HTTPRoutes), gatewayResources.HTTPRoutes)
			} else {
				for i, got := range gatewayResources.HTTPRoutes {
					want := tc.expectedGatewayResources.HTTPRoutes[i2gw.HTTPRouteToHTTPRouteKey(got)]
					want.SetGroupVersionKind(httpRouteGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected HTTPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(gatewayResources.Gateways) != len(tc.expectedGatewayResources.Gateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v",
					len(tc.expectedGatewayResources.Gateways), len(gatewayResources.Gateways), gatewayResources.Gateways)
			} else {
				for i, got := range gatewayResources.Gateways {
					want := tc.expectedGatewayResources.Gateways[i2gw.GatewayToGatewayKey(got)]
					want.SetGroupVersionKind(gatewayGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
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

func stringPtr(s string) *string {
	return &s
}

// TODO: Replace these with Gateway API util funcs.
func apiGroupPtr(s string) *gatewayv1beta1.Group {
	g := gatewayv1beta1.Group(s)
	return &g
}

func apiKindPtr(s string) *gatewayv1beta1.Kind {
	k := gatewayv1beta1.Kind(s)
	return &k
}

func portNumberPtr(p int) *gatewayv1beta1.PortNumber {
	pn := gatewayv1beta1.PortNumber(p)
	return &pn
}

func gatewayHostnamePtr(s string) *gatewayv1beta1.Hostname {
	h := gatewayv1beta1.Hostname(s)
	return &h
}
