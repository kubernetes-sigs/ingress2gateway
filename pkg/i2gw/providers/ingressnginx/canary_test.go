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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_ingressRuleGroup_calculateBackendRefWeight(t *testing.T) {
	testCases := []struct {
		name                string
		paths               []ingressPath
		expectedBackendRefs []gatewayv1.HTTPBackendRef
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
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canaryAnnotations{
						enable: true,
						weight: 101,
					}},
				},
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "prod",
								Kind:     "StorageBucket",
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1.HTTPBackendRef{
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(100))}},
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(0))}},
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
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canaryAnnotations{
						enable: true,
						weight: 30,
					}},
				},
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "prod",
								Kind:     "StorageBucket",
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1.HTTPBackendRef{
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(30))}},
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(70))}},
			},
		},
		{
			name: "set weight as 0",
			paths: []ingressPath{
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "canary",
								Kind:     "StorageBucket",
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canaryAnnotations{
						enable: true,
						weight: 0,
					}},
				},
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "prod",
								Kind:     "StorageBucket",
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1.HTTPBackendRef{
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(0))}},
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(100))}},
			},
		},
		{
			name: "set weight as 100",
			paths: []ingressPath{
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "canary",
								Kind:     "StorageBucket",
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canaryAnnotations{
						enable: true,
						weight: 100,
					}},
				},
				{
					path: networkingv1.HTTPIngressPath{
						Backend: networkingv1.IngressBackend{
							Resource: &corev1.TypedLocalObjectReference{
								Name:     "prod",
								Kind:     "StorageBucket",
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1.HTTPBackendRef{
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(100))}},
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(0))}},
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
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
					extra: &extra{canary: &canaryAnnotations{
						enable:      true,
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
								APIGroup: ptrTo("vendor.example.com"),
							},
						},
					},
				},
			},
			expectedBackendRefs: []gatewayv1.HTTPBackendRef{
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(50))}},
				{BackendRef: gatewayv1.BackendRef{Weight: ptrTo(int32(150))}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			actualBackendRefs, errs := calculateBackendRefWeight(tc.paths)
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

func Test_parseCanaryAnnotations(t *testing.T) {
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
				canary: &canaryAnnotations{
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
				canary: &canaryAnnotations{
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

			actualCanary, errs := parseCanaryAnnotations(tc.ingress)
			if len(errs) != len(tc.expectedError) {
				t.Fatalf("expected %d errors, got %d", len(tc.expectedError), len(errs))
			}

			if len(tc.expectedError) > 0 {
				return
			}

			expectedCanary := tc.expectedExtra.canary

			if diff := cmp.Diff(actualCanary, *expectedCanary, cmp.AllowUnexported(canaryAnnotations{})); diff != "" {
				t.Fatalf("getExtra() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
