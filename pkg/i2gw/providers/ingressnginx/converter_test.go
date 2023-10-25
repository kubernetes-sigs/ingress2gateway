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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func Test_ToGateway(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	//iExact := networkingv1.PathTypeExact
	gPathPrefix := gatewayv1beta1.PathMatchPathPrefix
	//gExact := gatewayv1beta1.PathMatchExact

	testCases := []struct {
		name                     string
		ingresses                []networkingv1.Ingress
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
	}{
		{
			name: "canary deployment",
			ingresses: []networkingv1.Ingress{
				{
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
											Resource: &corev1.TypedLocalObjectReference{
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
				{
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
											Resource: &corev1.TypedLocalObjectReference{
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1beta1.Gateway{
					{Namespace: "default", Name: "ingress-nginx"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "ingress-nginx", Namespace: "default"},
						Spec: gatewayv1beta1.GatewaySpec{
							GatewayClassName: "ingress-nginx",
							Listeners: []gatewayv1beta1.Listener{{
								Name:     "echo-prod-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1beta1.HTTPProtocolType,
								Hostname: ptrTo(gatewayv1beta1.Hostname("echo.prod.mydomain.com")),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1beta1.HTTPRoute{
					{Namespace: "default", Name: "production-echo-prod-mydomain-com"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "production-echo-prod-mydomain-com", Namespace: "default"},
						Spec: gatewayv1beta1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
								ParentRefs: []gatewayv1beta1.ParentReference{{
									Name: "ingress-nginx",
								}},
							},
							Hostnames: []gatewayv1beta1.Hostname{"echo.prod.mydomain.com"},
							Rules: []gatewayv1beta1.HTTPRouteRule{{
								Matches: []gatewayv1beta1.HTTPRouteMatch{{
									Path: &gatewayv1beta1.HTTPPathMatch{
										Type:  &gPathPrefix,
										Value: ptrTo("/"),
									},
								}},
								BackendRefs: []gatewayv1beta1.HTTPBackendRef{
									{
										BackendRef: gatewayv1beta1.BackendRef{
											BackendObjectReference: gatewayv1beta1.BackendObjectReference{
												Name:  "production",
												Group: ptrTo(gatewayv1beta1.Group("vendor.example.com")),
												Kind:  ptrTo(gatewayv1beta1.Kind("StorageBucket")),
											},
											Weight: ptrTo(int32(80)),
										},
									},
									{
										BackendRef: gatewayv1beta1.BackendRef{
											BackendObjectReference: gatewayv1beta1.BackendObjectReference{
												Name:  "canary",
												Group: ptrTo(gatewayv1beta1.Group("vendor.example.com")),
												Kind:  ptrTo(gatewayv1beta1.Kind("StorageBucket")),
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
			expectedErrors: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			provider := NewProvider(i2gw.ProviderConf{})

			resources := i2gw.InputResources{
				Ingresses:       tc.ingresses,
				CustomResources: nil,
			}

			gatewayResources, errs := provider.ToGatewayAPI(resources)

			if len(gatewayResources.HTTPRoutes) != len(tc.expectedGatewayResources.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.HTTPRoutes), len(gatewayResources.HTTPRoutes), gatewayResources.HTTPRoutes)
			} else {
				for i, got := range gatewayResources.HTTPRoutes {
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.HTTPRoutes[key]
					want.SetGroupVersionKind(common.HTTPRouteGVK)
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
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.Gateways[key]
					want.SetGroupVersionKind(common.GatewayGVK)
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

func ptrTo[T any](a T) *T {
	return &a
}
