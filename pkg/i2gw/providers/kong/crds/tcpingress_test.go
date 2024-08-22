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

package crds

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	kongv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestTCPIngressToGatewayAPI(t *testing.T) {
	// iPrefix := networkingv1.PathTypePrefix
	// gPathPrefix := gatewayv1beta1.PathMatchPathPrefix

	testCases := []struct {
		name                     string
		tcpIngresses             []kongv1beta1.TCPIngress
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
	}{
		{
			name: "TCPIngress to Gateway and TCPRoute",
			tcpIngresses: []kongv1beta1.TCPIngress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample",
						Namespace: "default",
						Annotations: map[string]string{
							"kubernetes.io/ingress.class": "kong",
						},
					},
					Spec: kongv1beta1.TCPIngressSpec{
						Rules: []kongv1beta1.IngressRule{
							{
								Port: 8888,
								Backend: kongv1beta1.IngressBackend{
									ServiceName: "tcp-echo",
									ServicePort: 1025,
								},
							},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "kong"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "kong", Namespace: "default"},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "kong",
							Listeners: []gatewayv1.Listener{{
								Name:     "tcp-all-hosts-8888",
								Port:     8888,
								Protocol: gatewayv1.TCPProtocolType,
							}},
						},
					},
				},
				TCPRoutes: map[types.NamespacedName]gatewayv1alpha2.TCPRoute{
					{Namespace: "default", Name: "sample-all-hosts"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sample-all-hosts",
							Namespace: "default",
						},
						Spec: gatewayv1alpha2.TCPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{
									{
										Name:        "kong",
										SectionName: common.PtrTo(gatewayv1.SectionName("tcp-all-hosts-8888")),
									},
								},
							},
							Rules: []gatewayv1alpha2.TCPRouteRule{
								{
									BackendRefs: []gatewayv1.BackendRef{
										{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "tcp-echo",
												Port: common.PtrTo(gatewayv1.PortNumber(1025)),
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
		{
			name: "TCPIngress to Gateway and TLSRoute",
			tcpIngresses: []kongv1beta1.TCPIngress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample",
						Namespace: "default",
						Annotations: map[string]string{
							"kubernetes.io/ingress.class": "kong",
						},
					},
					Spec: kongv1beta1.TCPIngressSpec{
						TLS: []kongv1beta1.IngressTLS{
							{
								SecretName: "testSecret",
							},
						},
						Rules: []kongv1beta1.IngressRule{
							{
								Port: 8888,
								Host: "example.com",
								Backend: kongv1beta1.IngressBackend{
									ServiceName: "tcp-echo",
									ServicePort: 1025,
								},
							},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "kong"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "kong", Namespace: "default"},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "kong",
							Listeners: []gatewayv1.Listener{{
								Name:     "tls-example-com-8888",
								Port:     8888,
								Protocol: gatewayv1.TLSProtocolType,
								Hostname: common.PtrTo(gatewayv1.Hostname("example.com")),
								TLS: &gatewayv1.GatewayTLSConfig{
									Mode: common.PtrTo(gatewayv1.TLSModePassthrough),
									CertificateRefs: []gatewayv1.SecretObjectReference{
										{
											Group: common.PtrTo(gatewayv1.Group("")),
											Kind:  common.PtrTo(gatewayv1.Kind("Secret")),
											Name:  "testSecret",
										},
									},
								},
							}},
						},
					},
				},
				TLSRoutes: map[types.NamespacedName]gatewayv1alpha2.TLSRoute{
					{Namespace: "default", Name: "sample-example-com"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sample-example-com",
							Namespace: "default",
						},
						Spec: gatewayv1alpha2.TLSRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{
									{
										Name:        "kong",
										SectionName: common.PtrTo(gatewayv1.SectionName("tls-example-com-8888")),
									},
								},
							},
							Rules: []gatewayv1alpha2.TLSRouteRule{
								{
									BackendRefs: []gatewayv1.BackendRef{
										{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "tcp-echo",
												Port: common.PtrTo(gatewayv1.PortNumber(1025)),
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir, _, errs := TCPIngressToGatewayIR(tc.tcpIngresses)

			if len(ir.Gateways) != len(tc.expectedGatewayResources.Gateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v",
					len(tc.expectedGatewayResources.Gateways), len(ir.Gateways), ir.Gateways)
			} else {
				for i, gotGatewayContext := range ir.Gateways {
					key := types.NamespacedName{Namespace: gotGatewayContext.Gateway.Namespace, Name: gotGatewayContext.Gateway.Name}
					want := tc.expectedGatewayResources.Gateways[key]
					want.SetGroupVersionKind(common.GatewayGVK)
					if !apiequality.Semantic.DeepEqual(gotGatewayContext.Gateway, want) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, want, gotGatewayContext.Gateway, cmp.Diff(want, gotGatewayContext.Gateway))
					}
				}
			}

			if len(ir.HTTPRoutes) != len(tc.expectedGatewayResources.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.HTTPRoutes), len(ir.HTTPRoutes), ir.HTTPRoutes)
			} else {
				for i, gotHTTPRouteContext := range ir.HTTPRoutes {
					key := types.NamespacedName{Namespace: gotHTTPRouteContext.HTTPRoute.Namespace, Name: gotHTTPRouteContext.HTTPRoute.Name}
					want := tc.expectedGatewayResources.HTTPRoutes[key]
					want.SetGroupVersionKind(common.HTTPRouteGVK)
					if !apiequality.Semantic.DeepEqual(gotHTTPRouteContext.HTTPRoute, want) {
						t.Errorf("Expected HTTPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, want, gotHTTPRouteContext.HTTPRoute, cmp.Diff(want, gotHTTPRouteContext.HTTPRoute))
					}
				}
			}

			if len(ir.TCPRoutes) != len(tc.expectedGatewayResources.TCPRoutes) {
				t.Errorf("Expected %d TCPRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.TCPRoutes), len(ir.TCPRoutes), ir.TCPRoutes)
			} else {
				for i, got := range ir.TCPRoutes {
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.TCPRoutes[key]
					want.SetGroupVersionKind(common.TCPRouteGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected TCPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(ir.TLSRoutes) != len(tc.expectedGatewayResources.TLSRoutes) {
				t.Errorf("Expected %d TLSRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.TLSRoutes), len(ir.TLSRoutes), ir.TLSRoutes)
			} else {
				for i, got := range ir.TLSRoutes {
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.TLSRoutes[key]
					want.SetGroupVersionKind(common.TLSRouteGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected TLSRoute %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
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
