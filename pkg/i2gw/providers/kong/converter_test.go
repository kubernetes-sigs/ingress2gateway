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

package kong

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_ToGateway(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	ImplSpecificPathType := networkingv1.PathTypeImplementationSpecific
	gPathPrefix := gatewayv1.PathMatchPathPrefix
	gPathRegex := gatewayv1.PathMatchRegularExpression

	testCases := []struct {
		name           string
		ingresses      map[types.NamespacedName]*networkingv1.Ingress
		expectedIR     providerir.ProviderIR
		expectedErrors field.ErrorList
	}{
		{
			name: "header matching, method matching, plugin, single ingress rule",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: "default", Name: "multiple-matching-single-rule"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-matching-single-rule",
						Namespace: "default",
						Annotations: map[string]string{
							"konghq.com/headers.key1": "val1",
							"konghq.com/methods":      "GET,POST",
							"konghq.com/plugins":      "plugin1",
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("ingress-kong"),
						Rules: []networkingv1.IngressRule{{
							Host: "test.mydomain.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
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
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "ingress-kong"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "ingress-kong", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "ingress-kong",
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
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "multiple-matching-single-rule-test-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "multiple-matching-single-rule-test-mydomain-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "ingress-kong",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"test.mydomain.com"},
								Rules: []gatewayv1.HTTPRouteRule{{
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
											Headers: []gatewayv1.HTTPHeaderMatch{
												{
													Name:  "key1",
													Value: "val1",
												},
											},
											Method: ptrTo(gatewayv1.HTTPMethodGet),
										},
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
											Headers: []gatewayv1.HTTPHeaderMatch{
												{
													Name:  "key1",
													Value: "val1",
												},
											},
											Method: ptrTo(gatewayv1.HTTPMethodPost),
										},
									},
									Filters: []gatewayv1.HTTPRouteFilter{
										{
											Type: gatewayv1.HTTPRouteFilterExtensionRef,
											ExtensionRef: &gatewayv1.LocalObjectReference{
												Group: gatewayv1.Group(kongResourcesGroup),
												Kind:  gatewayv1.Kind(kongPluginKind),
												Name:  gatewayv1.ObjectName("plugin1"),
											},
										},
									},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "test",
													Port: ptrTo(gatewayv1.PortNumber(80)),
												},
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
			name: "header matching, method matching, multiple ingress rules",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: "default", Name: "multiple-matching-multiple-rules"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-matching-multiple-rules",
						Namespace: "default",
						Annotations: map[string]string{
							"konghq.com/headers.key1": "val1",
							"konghq.com/methods":      "GET,POST",
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("ingress-kong"),
						Rules: []networkingv1.IngressRule{{
							Host: "test.mydomain.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/first",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "test-first",
													Port: networkingv1.ServiceBackendPort{
														Number: 80,
													},
												},
											},
										},
										{
											Path:     "/second",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "test-second",
													Port: networkingv1.ServiceBackendPort{
														Number: 80,
													},
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
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "ingress-kong"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "ingress-kong", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "ingress-kong",
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
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "multiple-matching-multiple-rules-test-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "multiple-matching-multiple-rules-test-mydomain-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "ingress-kong",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"test.mydomain.com"},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/first"),
												},
												Headers: []gatewayv1.HTTPHeaderMatch{
													{
														Name:  "key1",
														Value: "val1",
													},
												},
												Method: ptrTo(gatewayv1.HTTPMethodGet),
											},
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/first"),
												},
												Headers: []gatewayv1.HTTPHeaderMatch{
													{
														Name:  "key1",
														Value: "val1",
													},
												},
												Method: ptrTo(gatewayv1.HTTPMethodPost),
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "test-first",
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/second"),
												},
												Headers: []gatewayv1.HTTPHeaderMatch{
													{
														Name:  "key1",
														Value: "val1",
													},
												},
												Method: ptrTo(gatewayv1.HTTPMethodGet),
											},
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/second"),
												},
												Headers: []gatewayv1.HTTPHeaderMatch{
													{
														Name:  "key1",
														Value: "val1",
													},
												},
												Method: ptrTo(gatewayv1.HTTPMethodPost),
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "test-second",
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
			name: "ImplementationSpecific HTTPRouteMatching with regex",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: "default", Name: "implementation-specific-regex"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "implementation-specific-regex",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("ingress-kong"),
						Rules: []networkingv1.IngressRule{{
							Host: "test.mydomain.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/~/echo/**/test",
										PathType: &ImplSpecificPathType,
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
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "ingress-kong"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "ingress-kong", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "ingress-kong",
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
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "implementation-specific-regex-test-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "implementation-specific-regex-test-mydomain-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "ingress-kong",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"test.mydomain.com"},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathRegex,
													Value: ptrTo("/echo/**/test"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "test",
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
			name: "ImplementationSpecific HTTPRouteMatching without regex",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: "default", Name: "implementation-no-regex"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "implementation-specific-no-regex",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("ingress-kong"),
						Rules: []networkingv1.IngressRule{{
							Host: "test.mydomain.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/echo",
										PathType: &ImplSpecificPathType,
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
			expectedIR: providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					{Namespace: "default", Name: "ingress-kong"}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: "ingress-kong", Namespace: "default"},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: "ingress-kong",
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
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "implementation-specific-no-regex-test-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "implementation-specific-no-regex-test-mydomain-com", Namespace: "default"},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: "ingress-kong",
									}},
								},
								Hostnames: []gatewayv1.Hostname{"test.mydomain.com"},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/echo"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "test",
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			provider := NewProvider(&i2gw.ProviderConf{})
			kongProvider := provider.(*Provider)
			kongProvider.storage = newResourceStorage()
			kongProvider.storage.Ingresses = tc.ingresses

			ir, errs := provider.ToIR()

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

func ptrTo[T any](a T) *T {
	return &a
}
