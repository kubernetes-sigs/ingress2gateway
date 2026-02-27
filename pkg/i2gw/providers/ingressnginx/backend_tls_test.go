/*
Copyright 2026 The Kubernetes Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestBackendTLSFeature(t *testing.T) {
	testCases := []struct {
		name                   string
		ingress                networkingv1.Ingress
		expectedPolicies       map[types.NamespacedName]gatewayv1.BackendTLSPolicy
		expectedPolicyTargeted bool // if false, expectedPolicies should be empty
	}{
		{
			name: "ssl-verify on",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ssl-verify",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
						"nginx.ingress.kubernetes.io/proxy-ssl-verify": "on",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			expectedPolicies:       nil,
			expectedPolicyTargeted: false,
		},
		{
			name: "ssl-secret provided",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ssl-secret",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
						"nginx.ingress.kubernetes.io/proxy-ssl-secret": "default/secret-valid",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			expectedPolicies:       nil,
			expectedPolicyTargeted: false,
		},
		{
			name: "ssl-secret and verify on",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ssl-secret-verify",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
						"nginx.ingress.kubernetes.io/proxy-ssl-secret": "secret-valid",
						"nginx.ingress.kubernetes.io/proxy-ssl-verify": "on",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			expectedPolicies:       nil,
			expectedPolicyTargeted: false,
		},
		{
			name: "backend-protocol HTTPS only",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "https-protocol",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			expectedPolicies:       nil,
			expectedPolicyTargeted: false,
		},
		{
			name: "backend-protocol HTTPS and proxy-ssl-name",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "https-protocol-ssl-name",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
						"nginx.ingress.kubernetes.io/proxy-ssl-name":   "custom.internal.com",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			expectedPolicies:       nil,
			expectedPolicyTargeted: false,
		},
		{
			name: "fully compliant strict TLS validation",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "strict-tls",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol":      "HTTPS",
						"nginx.ingress.kubernetes.io/proxy-ssl-verify":      "on",
						"nginx.ingress.kubernetes.io/proxy-ssl-secret":      "my-ca-secret",
						"nginx.ingress.kubernetes.io/proxy-ssl-server-name": "on",
						"nginx.ingress.kubernetes.io/proxy-ssl-name":        "strict.internal.com",
						// proxy-ssl-server-name defaults to "on" implicitly, but could be added here
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			expectedPolicies: map[types.NamespacedName]gatewayv1.BackendTLSPolicy{
				{Namespace: "default", Name: "test-service-backend-tls"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service-backend-tls",
						Namespace: "default",
					},
					Spec: gatewayv1.BackendTLSPolicySpec{
						TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{{
							LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  "test-service",
							},
						}},
						Validation: gatewayv1.BackendTLSPolicyValidation{
							Hostname: gatewayv1.PreciseHostname("strict.internal.com"),
							CACertificateRefs: []gatewayv1.LocalObjectReference{{
								Group: "",
								Kind:  "Secret",
								Name:  "my-ca-secret",
							}},
						},
					},
				},
			},
			expectedPolicyTargeted: true,
		},
		{
			name: "no relevant annotations",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "normal-ingress",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			expectedPolicies:       nil,
			expectedPolicyTargeted: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes:         make(map[types.NamespacedName]providerir.HTTPRouteContext),
				BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy),
			}

			// Replicate IR setup
			key := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: common.RouteName(tc.ingress.Name, "example.com")}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tc.ingress.Namespace,
					Name:      key.Name,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: gatewayv1.ObjectName("test-service"),
											Kind: ptr.To(gatewayv1.Kind("Service")),
										},
									},
								},
							},
						},
					},
				},
			}
			ir.HTTPRoutes[key] = providerir.HTTPRouteContext{
				HTTPRoute: route,
				RuleBackendSources: [][]providerir.BackendSource{
					{
						{Ingress: &tc.ingress},
					},
				},
			}

			errs := backendTLSFeature(notifications.NoopNotify, []networkingv1.Ingress{tc.ingress}, nil, &ir)
			if len(errs) > 0 {
				t.Fatalf("Expected no errors, got %v", errs)
			}

			if !tc.expectedPolicyTargeted {
				if len(ir.BackendTLSPolicies) > 0 {
					t.Errorf("Expected no BackendTLSPolicies, got %d", len(ir.BackendTLSPolicies))
				}
				return
			}

			if len(ir.BackendTLSPolicies) != len(tc.expectedPolicies) {
				t.Errorf("Expected %d BackendTLSPolicies, got %d", len(tc.expectedPolicies), len(ir.BackendTLSPolicies))
			}

			for key, wantPolicy := range tc.expectedPolicies {
				gotPolicy, ok := ir.BackendTLSPolicies[key]
				if !ok {
					t.Errorf("Expected BackendTLSPolicy %s not found", key)
					continue
				}

				// Manually set GVK for comparison if needed, or rely on deep equal of fields
				// common.CreateBackendTLSPolicy sets GVK roughly, but let's check deep equal of Spec
				if !apiequality.Semantic.DeepEqual(gotPolicy.Spec, wantPolicy.Spec) {
					t.Errorf("BackendTLSPolicy Spec mismatch (-want +got):\n%s", cmp.Diff(wantPolicy.Spec, gotPolicy.Spec))
				}
			}
		})
	}
}

func TestBackendTLSFeatureExtended(t *testing.T) {
	testCases := []struct {
		name                   string
		ingresses              []networkingv1.Ingress
		expectedPolicies       map[types.NamespacedName]gatewayv1.BackendTLSPolicy
		expectedPolicyTargeted bool
	}{
		{
			name: "canary ingress skipped",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "canary-ingress",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol":      "HTTPS",
							"nginx.ingress.kubernetes.io/proxy-ssl-verify":      "on",
							"nginx.ingress.kubernetes.io/proxy-ssl-secret":      "my-ca-secret",
							"nginx.ingress.kubernetes.io/proxy-ssl-server-name": "on",
							"nginx.ingress.kubernetes.io/proxy-ssl-name":        "strict.internal.com",
							"nginx.ingress.kubernetes.io/canary":                "true",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: ptr.To(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test-service",
												Port: networkingv1.ServiceBackendPort{Number: 80},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			expectedPolicies: map[types.NamespacedName]gatewayv1.BackendTLSPolicy{
				{Namespace: "default", Name: "test-service-backend-tls"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service-backend-tls",
						Namespace: "default",
					},
					Spec: gatewayv1.BackendTLSPolicySpec{
						TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{{
							LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  "test-service",
							},
						}},
						Validation: gatewayv1.BackendTLSPolicyValidation{
							Hostname: gatewayv1.PreciseHostname("strict.internal.com"),
							CACertificateRefs: []gatewayv1.LocalObjectReference{{
								Group: "",
								Kind:  "Secret",
								Name:  "my-ca-secret",
							}},
						},
					},
				},
			},
			expectedPolicyTargeted: true,
		},
		{
			name: "cross-namespace secret skipped",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cross-ns-secret",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
							"nginx.ingress.kubernetes.io/proxy-ssl-secret": "other-ns/secret",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: ptr.To(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test-service",
												Port: networkingv1.ServiceBackendPort{Number: 80},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			expectedPolicies:       nil,
			expectedPolicyTargeted: false,
		},
		{
			name: "conflict - first wins",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ingress-1",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol":      "HTTPS",
							"nginx.ingress.kubernetes.io/proxy-ssl-verify":      "on",
							"nginx.ingress.kubernetes.io/proxy-ssl-secret":      "my-ca-secret",
							"nginx.ingress.kubernetes.io/proxy-ssl-server-name": "on",
							"nginx.ingress.kubernetes.io/proxy-ssl-name":        "first.com",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: ptr.To(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test-service",
												Port: networkingv1.ServiceBackendPort{Number: 80},
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
						Name:      "ingress-2",
						Namespace: "default",
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/backend-protocol":      "HTTPS",
							"nginx.ingress.kubernetes.io/proxy-ssl-verify":      "on",
							"nginx.ingress.kubernetes.io/proxy-ssl-secret":      "my-ca-secret",
							"nginx.ingress.kubernetes.io/proxy-ssl-server-name": "on",
							"nginx.ingress.kubernetes.io/proxy-ssl-name":        "second.com", // Conflict
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/bar",
										PathType: ptr.To(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test-service", // Same service
												Port: networkingv1.ServiceBackendPort{Number: 80},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			expectedPolicies: map[types.NamespacedName]gatewayv1.BackendTLSPolicy{
				{Namespace: "default", Name: "test-service-backend-tls"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service-backend-tls",
						Namespace: "default",
					},
					Spec: gatewayv1.BackendTLSPolicySpec{
						TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{{
							LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  "test-service",
							},
						}},
						Validation: gatewayv1.BackendTLSPolicyValidation{
							Hostname: gatewayv1.PreciseHostname("first.com"), // Expect first one
							CACertificateRefs: []gatewayv1.LocalObjectReference{{
								Group: "",
								Kind:  "Secret",
								Name:  "my-ca-secret",
							}},
						},
					},
				},
			},
			expectedPolicyTargeted: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes:         make(map[types.NamespacedName]providerir.HTTPRouteContext),
				BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy),
			}

			// Replicate IR setup for multiple ingresses
			for i := range tc.ingresses {
				ing := tc.ingresses[i]
				key := types.NamespacedName{Namespace: ing.Namespace, Name: common.RouteName(ing.Name, "example.com")}

				// Simplified route setup
				route := gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{Namespace: ing.Namespace, Name: key.Name},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{{
							BackendRefs: []gatewayv1.HTTPBackendRef{{
								BackendRef: gatewayv1.BackendRef{
									BackendObjectReference: gatewayv1.BackendObjectReference{
										Name: "test-service",
										Kind: ptr.To(gatewayv1.Kind("Service")),
									},
								},
							}},
						}},
					},
				}

				ir.HTTPRoutes[key] = providerir.HTTPRouteContext{
					HTTPRoute: route,
					RuleBackendSources: [][]providerir.BackendSource{
						{{Ingress: &tc.ingresses[i]}},
					},
				}
			}

			errs := backendTLSFeature(notifications.NoopNotify, tc.ingresses, nil, &ir)
			if len(errs) > 0 {
				t.Fatalf("Expected no errors, got %v", errs)
			}

			if !tc.expectedPolicyTargeted {
				if len(ir.BackendTLSPolicies) > 0 {
					t.Errorf("Expected no BackendTLSPolicies, got %d", len(ir.BackendTLSPolicies))
				}
				return
			}

			if len(ir.BackendTLSPolicies) != len(tc.expectedPolicies) {
				t.Errorf("Expected %d BackendTLSPolicies, got %d", len(tc.expectedPolicies), len(ir.BackendTLSPolicies))
			}

			for key, wantPolicy := range tc.expectedPolicies {
				gotPolicy, ok := ir.BackendTLSPolicies[key]
				if !ok {
					t.Errorf("Expected BackendTLSPolicy %s not found", key)
					continue
				}

				if !apiequality.Semantic.DeepEqual(gotPolicy.Spec, wantPolicy.Spec) {
					t.Errorf("BackendTLSPolicy Spec mismatch (-want +got):\n%s", cmp.Diff(wantPolicy.Spec, gotPolicy.Spec))
				}
			}
		})
	}
}
