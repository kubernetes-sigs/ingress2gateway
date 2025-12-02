/*
Copyright 2024 The Kubernetes Authors.

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
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_ToGatewayResources(t *testing.T) {
	gPathPrefix := gatewayv1.PathMatchPathPrefix

	testCases := []struct {
		desc                     string
		ir                       intermediate.IR
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
	}{
		{
			desc:                     "empty",
			ir:                       intermediate.IR{},
			expectedGatewayResources: i2gw.GatewayResources{},
			expectedErrors:           field.ErrorList{},
		},
		{
			desc: "no additional extensions",
			ir: intermediate.IR{
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "test", Name: "simple"}: {
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
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "test", Name: "simple-example-com"}: {
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
		{
			desc: "duplicated backends",
			ir: intermediate.IR{
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
					{Namespace: "test", Name: "duplicate-example-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "duplicate-example-com", Namespace: "test"},
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
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "example",
													Port: PtrTo(gatewayv1.PortNumber(3000)),
												},
											},
										},
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "example",
													Port: PtrTo(gatewayv1.PortNumber(3000)),
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "test", Name: "example-proxy"}: {
						TypeMeta: metav1.TypeMeta{
							Kind:       "Gateway",
							APIVersion: "gateway.networking.k8s.io/v1",
						},
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
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "test", Name: "duplicate-example-com"}: {
						TypeMeta: metav1.TypeMeta{
							Kind:       "HTTPRoute",
							APIVersion: "gateway.networking.k8s.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{Name: "duplicate-example-com", Namespace: "test"},
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
								BackendRefs: []gatewayv1.HTTPBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "example",
												Port: PtrTo(gatewayv1.PortNumber(3000)),
											},
										},
									},
								},
							}},
						},
						Status: gatewayv1.HTTPRouteStatus{
							RouteStatus: gatewayv1.RouteStatus{
								Parents: []gatewayv1.RouteParentStatus{},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gatewayResouces, errs := ToGatewayResources(tc.ir)

			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
			}

			if len(gatewayResouces.HTTPRoutes) != len(tc.expectedGatewayResources.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.HTTPRoutes), len(gatewayResouces.HTTPRoutes), gatewayResouces.HTTPRoutes)
			} else {
				for i, got := range gatewayResouces.HTTPRoutes {
					got.SetGroupVersionKind(HTTPRouteGVK)
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.HTTPRoutes[key]
					want.SetGroupVersionKind(HTTPRouteGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected HTTPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(gatewayResouces.Gateways) != len(tc.expectedGatewayResources.Gateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v",
					len(tc.expectedGatewayResources.Gateways), len(gatewayResouces.Gateways), gatewayResouces.Gateways)
			} else {
				for i, got := range gatewayResouces.Gateways {
					got.SetGroupVersionKind(GatewayGVK)
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.Gateways[key]
					want.SetGroupVersionKind(GatewayGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}
		})
	}
}
