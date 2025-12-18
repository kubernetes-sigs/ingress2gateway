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

package apisix

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_httpToHttpsFeature(t *testing.T) {
	testCases := []struct {
		name              string
		ingress           networkingv1.Ingress
		initialHTTPRoute  *gatewayv1.HTTPRoute
		expectedHTTPRoute *gatewayv1.HTTPRoute
		expectedError     field.ErrorList
	}{
		{
			name: "annotation present",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						"k8s.apisix.apache.org/http-to-https": "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
						},
					},
				},
			},
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterRequestRedirect,
									RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
										Scheme:     ptr.To("https"),
										StatusCode: ptr.To(int(301)),
									},
								},
							},
						},
					},
				},
			},
			expectedError: field.ErrorList{},
		},
		{
			name: "annotation not present",
			ingress: networkingv1.Ingress{
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
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedError: field.ErrorList{},
		},
		{
			name: "annotation present but false",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						"k8s.apisix.apache.org/http-to-https": "false",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
						},
					},
				},
			},
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedError: field.ErrorList{},
		},
		{
			name: "annotation present but invalid value",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						"k8s.apisix.apache.org/http-to-https": "invalid",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
						},
					},
				},
			},
			initialHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedHTTPRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress-example-com",
					Namespace: "default",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "example", Port: ptr.To(gatewayv1.PortNumber(3000))}}},
							},
						},
					},
				},
			},
			expectedError: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingresses := []networkingv1.Ingress{tc.ingress}
			ir := &providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Name: tc.expectedHTTPRoute.Name, Namespace: tc.expectedHTTPRoute.Namespace}: {
						HTTPRoute: *tc.initialHTTPRoute,
					},
				},
			}

			errs := httpToHTTPSFeature(ingresses, map[types.NamespacedName]map[string]int32{}, ir, nil)

			if len(errs) != len(tc.expectedError) {
				t.Errorf("expected %d errors, got %d", len(tc.expectedError), len(errs))
			}

			key := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: common.RouteName(tc.ingress.Name, tc.ingress.Spec.Rules[0].Host)}

			actualHTTPRouteContext, ok := ir.HTTPRoutes[key]
			if !ok {
				t.Errorf("HTTPRoute not found: %v", key)
			}

			if diff := cmp.Diff(*tc.expectedHTTPRoute, actualHTTPRouteContext.HTTPRoute); diff != "" {
				t.Errorf("Unexpected HTTPRoute resource found, \n want: %+v\n got: %+v\n diff (-want +got):\n%s", *tc.expectedHTTPRoute, actualHTTPRouteContext.HTTPRoute, diff)
			}
		})
	}
}
