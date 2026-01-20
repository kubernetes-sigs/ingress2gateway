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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Test_gceFeature(t *testing.T) {
	testService := "test-service"
	testNamespace := "default"
	testCookieName := "MY_SESSION_COOKIE"
	testCookieTTL := int64(3600)

	testCases := []struct {
		name       string
		ingresses  []networkingv1.Ingress
		initialIR  *providerir.ProviderIR
		expectedIR *providerir.ProviderIR
	}{
		{
			name: "ingress with session-cookie-name and expires annotation",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress",
						Namespace: testNamespace,
						Annotations: map[string]string{
							SessionAffinityAnnotation:      "cookie",
							SessionCookieNameAnnotation:    testCookieName,
							SessionCookieExpiresAnnotation: "3600",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: testService,
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
			initialIR: &providerir.ProviderIR{
				Services: make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
			},
			expectedIR: &providerir.ProviderIR{
				Services: map[types.NamespacedName]providerir.ProviderSpecificServiceIR{
					{Namespace: testNamespace, Name: testService}: {
						Gce: &gce.ServiceIR{
							SessionAffinity: &gce.SessionAffinityConfig{
								AffinityType: "GENERATED_COOKIE",
								CookieName:   testCookieName,
								CookieTTLSec: &testCookieTTL,
							},
						},
					},
				},
			},
		},
		{
			name: "ingress without session affinity annotation",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress-no-affinity",
						Namespace: testNamespace,
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: testService,
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
			initialIR: &providerir.ProviderIR{
				Services: make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
			},
			expectedIR: &providerir.ProviderIR{
				Services: map[types.NamespacedName]providerir.ProviderSpecificServiceIR{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = gceFeature(tc.ingresses, nil, tc.initialIR)

			// Helper to compare Service IRs
			// We only check for the specific service we are testing
			key := types.NamespacedName{Namespace: testNamespace, Name: testService}
			
			gotServiceIR, ok := tc.initialIR.Services[key]
			expectedServiceIR, expectedOk := tc.expectedIR.Services[key]

			if !ok && !expectedOk {
				return // Both nil, matches expected empty
			}
			if ok != expectedOk {
				t.Errorf("Service IR existence mismatch. Got: %v, Expected: %v", ok, expectedOk)
				return
			}

			if diff := cmp.Diff(expectedServiceIR.Gce, gotServiceIR.Gce); diff != "" {
				t.Errorf("gceFeature() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
