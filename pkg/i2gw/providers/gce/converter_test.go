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

package gce

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_ToGateway(t *testing.T) {
	testNamespace := "default"
	testHost := "test.mydomain.com"
	testBackendServiceName := "test"
	iPrefix := networkingv1.PathTypePrefix
	implSpecificPathType := networkingv1.PathTypeImplementationSpecific

	gPathPrefix := gatewayv1.PathMatchPathPrefix
	gExact := gatewayv1.PathMatchExact

	extIngClassIngressName := "gce-ingress-class"
	intIngClassIngressName := "gce-internal-ingress-class"
	noIngClassIngressName := "no-ingress-class"

	testCases := []struct {
		name                     string
		ingresses                map[types.NamespacedName]*networkingv1.Ingress
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
	}{
		{
			name: "gce ingress class",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        extIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testBackendServiceName,
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
							Listeners: []gatewayv1.Listener{{
								Name:     "test-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
								Hostname: ptrTo(gatewayv1.Hostname(testHost)),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: gceIngressClass,
								}},
							},
							Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
										},
									},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: gatewayv1.ObjectName(testBackendServiceName),
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
			expectedErrors: field.ErrorList{},
		},
		{
			name: "gce-internal ingress class",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: intIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        intIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceL7ILBIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testBackendServiceName,
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: gceL7ILBIngressClass}: {
						ObjectMeta: metav1.ObjectMeta{Name: gceL7ILBIngressClass, Namespace: testNamespace},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: gceL7RegionalInternalGatewayClass,
							Listeners: []gatewayv1.Listener{{
								Name:     "test-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
								Hostname: ptrTo(gatewayv1.Hostname(testHost)),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: "gce-internal-ingress-class-test-mydomain-com"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "gce-internal-ingress-class-test-mydomain-com", Namespace: testNamespace},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: gceL7ILBIngressClass,
								}},
							},
							Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
										},
									},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: gatewayv1.ObjectName(testBackendServiceName),
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
			expectedErrors: field.ErrorList{},
		},
		{
			name: "no ingress class, default to gce ingress class behavior",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: noIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        noIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testBackendServiceName,
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
							Listeners: []gatewayv1.Listener{{
								Name:     "test-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
								Hostname: ptrTo(gatewayv1.Hostname(testHost)),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", noIngClassIngressName)}: {
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", noIngClassIngressName), Namespace: testNamespace},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: gceIngressClass,
								}},
							},
							Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
										},
									},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: gatewayv1.ObjectName(testBackendServiceName),
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
			expectedErrors: field.ErrorList{},
		},
		{
			name: "gce implementation-specific with /*, map to / Prefix",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        extIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/*",
										PathType: &implSpecificPathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testBackendServiceName,
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
							Listeners: []gatewayv1.Listener{{
								Name:     "test-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
								Hostname: ptrTo(gatewayv1.Hostname(testHost)),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: gceIngressClass,
								}},
							},
							Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/"),
											},
										},
									},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: gatewayv1.ObjectName(testBackendServiceName),
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
			expectedErrors: field.ErrorList{},
		},
		{
			name: "gce implementation-specific with /foo/*, converted to /foo",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        extIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/foo/*",
										PathType: &implSpecificPathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testBackendServiceName,
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
							Listeners: []gatewayv1.Listener{{
								Name:     "test-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
								Hostname: ptrTo(gatewayv1.Hostname(testHost)),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: gceIngressClass,
								}},
							},
							Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gPathPrefix,
												Value: ptrTo("/foo"),
											},
										},
									},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: gatewayv1.ObjectName(testBackendServiceName),
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
			expectedErrors: nil,
		},
		{
			name: "gce implementation-specific without wildcard path, map to Prefix",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        extIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/foo",
										PathType: &implSpecificPathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testBackendServiceName,
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
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
							Listeners: []gatewayv1.Listener{{
								Name:     "test-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
								Hostname: ptrTo(gatewayv1.Hostname(testHost)),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: gceIngressClass,
								}},
							},
							Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  &gExact,
												Value: ptrTo("/foo"),
											},
										},
									},
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: gatewayv1.ObjectName(testBackendServiceName),
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
			expectedErrors: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			provider := NewProvider(&i2gw.ProviderConf{})
			gceProvider := provider.(*Provider)
			gceProvider.storage = newResourcesStorage()
			gceProvider.storage.Ingresses = tc.ingresses

			// TODO(#113) we pass an empty i2gw.InputResources temporarily until we change ToGatewayAPI function on the interface
			gatewayResources, errs := provider.ToGatewayAPI()

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
