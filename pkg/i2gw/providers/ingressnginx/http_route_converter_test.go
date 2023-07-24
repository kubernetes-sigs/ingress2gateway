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

package ingressnginx

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func Test_ingresses2GatewaysAndHttpRoutes(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	iExact := networkingv1.PathTypeExact
	gPathPrefix := gatewayv1beta1.PathMatchPathPrefix
	gExact := gatewayv1beta1.PathMatchExact

	testCases := []struct {
		name             string
		ingresses        []networkingv1.Ingress
		expectGateways   []gatewayv1beta1.Gateway
		expectHTTPRoutes []gatewayv1beta1.HTTPRoute
		expectErrors     []error
	}{{
		name:             "empty",
		ingresses:        []networkingv1.Ingress{},
		expectGateways:   []gatewayv1beta1.Gateway{},
		expectHTTPRoutes: []gatewayv1beta1.HTTPRoute{},
		expectErrors:     []error{},
	}, {
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
		expectGateways: []gatewayv1beta1.Gateway{{
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
		}},
		expectHTTPRoutes: []gatewayv1beta1.HTTPRoute{{
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
		}},
		expectErrors: []error{},
	}, {
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
		expectGateways: []gatewayv1beta1.Gateway{{
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
		}},
		expectHTTPRoutes: []gatewayv1beta1.HTTPRoute{{
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
		}},
		expectErrors: []error{},
	}, {
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
		expectGateways: []gatewayv1beta1.Gateway{{
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
		}},
		expectHTTPRoutes: []gatewayv1beta1.HTTPRoute{{
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
		}, {
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
		}},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aggregator := ingressAggregator{ruleGroups: map[ruleGroupKey]*ingressRuleGroup{}}

			for _, ingress := range tc.ingresses {
				aggregator.addIngress(ingress)
			}

			httpRoutes, gateways, errs := aggregator.toHTTPRoutesAndGateways()

			if len(httpRoutes) != len(tc.expectHTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v", len(tc.expectHTTPRoutes), len(httpRoutes), httpRoutes)
			} else {
				for i, got := range httpRoutes {
					want := tc.expectHTTPRoutes[i]
					want.SetGroupVersionKind(httpRouteGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected HTTPRoute %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(gateways) != len(tc.expectGateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v", len(tc.expectGateways), len(gateways), gateways)
			} else {
				for i, got := range gateways {
					want := tc.expectGateways[i]
					want.SetGroupVersionKind(gatewayGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected Gateway %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(errs) != len(tc.expectErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectErrors[i])
					}
				}
			}
		})
	}
}

func int32Ptr(n int32) *int32 {
	return &n
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

func Test_getExtra(t *testing.T) {
	testCases := []struct {
		name          string
		ingress       networkingv1.Ingress
		expectedExtra *extra
		expectedError field.ErrorList
	}{
		{
			name: "actually get weights",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "50",
						"nginx.ingress.kubernetes.io/canary-weight-total": "100",
					},
				},
			},
			expectedExtra: &extra{
				canary: &canary{
					enable:      true,
					weight:      50,
					weightTotal: 100,
				},
			},
		},
		{
			name: "assigns default weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "50",
					},
				},
			},
			expectedExtra: &extra{
				canary: &canary{
					enable:      true,
					weight:      50,
					weightTotal: 100,
				},
			},
		},
		{
			name: "errors on non integer weight",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "50.5",
					},
				},
			},
			expectedExtra: &extra{},
			expectedError: field.ErrorList{field.TypeInvalid(field.NewPath(""), "", "")},
		},
		{
			name: "errors on non integer weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "50.5",
					},
				},
			},
			expectedExtra: &extra{},
			expectedError: field.ErrorList{field.TypeInvalid(field.NewPath(""), "", "")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			actualExtra, errs := getExtra(tc.ingress)
			if len(errs) != len(tc.expectedError) {
				t.Fatalf("expected %d errors, got %d", len(tc.expectedError), len(errs))
			}

			if len(tc.expectedError) > 0 {
				return
			}

			actualCanary := actualExtra.canary
			expectedCanary := tc.expectedExtra.canary

			if diff := cmp.Diff(*actualCanary, *expectedCanary, cmp.AllowUnexported(canary{})); diff != "" {
				t.Fatalf("getExtra() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_ingressRuleGroup_calculateBackendRefWeight(t *testing.T) {
	testCases := []struct {
		name                string
		paths               []ingressPath
		expectedBackendRefs []gatewayv1beta1.HTTPBackendRef
		expectedErrors      field.ErrorList
	}{
		{
			name: "respect weight boundaries",
			paths: []ingressPath{
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "canary",
								Kind:     "StorageBucket",
								APIGroup: stringPtr("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canary{
						weight: 101,
					}},
				},
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "prod",
								Kind:     "StorageBucket",
								APIGroup: stringPtr("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1beta1.HTTPBackendRef{
				{BackendRef: gatewayv1beta1.BackendRef{Weight: int32Ptr(100)}},
				{BackendRef: gatewayv1beta1.BackendRef{Weight: int32Ptr(0)}},
			},
		},
		{
			name: "default total weight",
			paths: []ingressPath{
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "canary",
								Kind:     "StorageBucket",
								APIGroup: stringPtr("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canary{
						weight: 30,
					}},
				},
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "prod",
								Kind:     "StorageBucket",
								APIGroup: stringPtr("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1beta1.HTTPBackendRef{
				{BackendRef: gatewayv1beta1.BackendRef{Weight: int32Ptr(30)}},
				{BackendRef: gatewayv1beta1.BackendRef{Weight: int32Ptr(70)}},
			},
		},
		{
			name: "weight total assigned",
			paths: []ingressPath{
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "canary",
								Kind:     "StorageBucket",
								APIGroup: stringPtr("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canary{
						weight:      50,
						weightTotal: 200,
					}},
				},
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "prod",
								Kind:     "StorageBucket",
								APIGroup: stringPtr("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1beta1.HTTPBackendRef{
				{BackendRef: gatewayv1beta1.BackendRef{Weight: int32Ptr(50)}},
				{BackendRef: gatewayv1beta1.BackendRef{Weight: int32Ptr(150)}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			var irg ingressRuleGroup
			actualBackendRefs, errs := irg.calculateBackendRefWeight(tc.paths)
			if len(errs) != len(tc.expectedErrors) {
				t.Fatalf("expected %d errors, got %d", len(tc.expectedErrors), len(errs))
			}

			if len(actualBackendRefs) != len(tc.expectedBackendRefs) {
				t.Fatalf("expected %d backend refs, got %d", len(tc.expectedBackendRefs), len(actualBackendRefs))
			}
			for i := 0; i < len(tc.expectedBackendRefs); i++ {
				if *tc.expectedBackendRefs[i].Weight != *actualBackendRefs[i].Weight {
					t.Fatalf("%s backendRef expected weight is %d, actual %d",
						actualBackendRefs[i].Name, *tc.expectedBackendRefs[i].Weight, *actualBackendRefs[i].Weight)
				}
			}
		})
	}
}
