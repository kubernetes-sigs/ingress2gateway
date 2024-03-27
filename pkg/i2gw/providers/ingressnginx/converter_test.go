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
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_ToGateway(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	//iExact := networkingv1.PathTypeExact
	isPathType := networkingv1.PathTypeImplementationSpecific
	gPathPrefix := gatewayv1.PathMatchPathPrefix
	//gExact := gatewayv1.PathMatchExact

	testCases := []struct {
		name                     string
		ingresses                OrderedIngressMap
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
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
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "ingress-nginx"}: {
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
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "default", Name: "production-echo-prod-mydomain-com"}: {
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
			expectedGatewayResources: i2gw.GatewayResources{},
			expectedErrors: field.ErrorList{
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.rules[0].http.paths[0].pathType",
					BadValue: ptr.To("ImplementationSpecific"),
					Detail:   "implementationSpecific path type is not supported in generic translation, and your provider does not provide custom support to translate it",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			provider := NewProvider(&i2gw.ProviderConf{})

			nginxProvider := provider.(*Provider)
			nginxProvider.storage.Ingresses = tc.ingresses

			// TODO(#113) we pass an empty i2gw.InputResources temporarily until we change ToGatewayAPI function on the interface
			gatewayResources, errs := provider.ToGatewayAPI(i2gw.InputResources{})

			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
			}

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
		})
	}
}

func ptrTo[T any](a T) *T {
	return &a
}
