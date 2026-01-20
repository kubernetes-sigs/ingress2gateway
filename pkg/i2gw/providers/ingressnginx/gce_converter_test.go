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
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestGCEFeature(t *testing.T) {
	testCases := []struct {
		name          string
		ingress       networkingv1.Ingress
		expectedGCE   *gce.ServiceIR
	}{
		{
			name: "No Affinity",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-affinity",
				},
			},
			expectedGCE: nil, // Should not modify service
		},
		{
			name: "Cookie Affinity",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cookie-affinity",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/affinity": "cookie",
					},
				},
			},
			expectedGCE: &gce.ServiceIR{
				SessionAffinity: &gce.SessionAffinityConfig{
					AffinityType: "GENERATED_COOKIE",
				},
			},
		},
		{
			name: "Cookie Affinity with Max Age",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cookie-affinity-maxlen",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/affinity":               "cookie",
						"nginx.ingress.kubernetes.io/session-cookie-max-age": "3600",
					},
				},
			},
			expectedGCE: &gce.ServiceIR{
				SessionAffinity: &gce.SessionAffinityConfig{
					AffinityType: "GENERATED_COOKIE",
					CookieTTLSec: ptr.To(int64(3600)),
				},
			},
		},
		{
			name: "Whitelist Source Range",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "whitelist",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/whitelist-source-range": "10.0.0.0/8",
					},
				},
			},
			expectedGCE: &gce.ServiceIR{
				SecurityPolicy: &gce.SecurityPolicyConfig{
					Name: "generated-whitelist-my-service",
				},
			},
		},
		{
			name: "Whitelist and Affinity Mixed",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "whitelist-affinity",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/affinity":               "cookie",
						"nginx.ingress.kubernetes.io/whitelist-source-range": "1.2.3.4/32",
					},
				},
			},
			expectedGCE: &gce.ServiceIR{
				SessionAffinity: &gce.SessionAffinityConfig{
					AffinityType: "GENERATED_COOKIE",
				},
				SecurityPolicy: &gce.SecurityPolicyConfig{
					Name: "generated-whitelist-my-service",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
				Services:   make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
			}

			// Mock Service
			svcKey := types.NamespacedName{Namespace: "default", Name: "my-service"}
			ir.Services[svcKey] = providerir.ProviderSpecificServiceIR{}

			// Mock Route Logic (Simplified to match gceFeature expectations)
			key := types.NamespacedName{Namespace: "default", Name: "test"}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "my-service",
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

			gceFeature([]networkingv1.Ingress{tc.ingress}, nil, &ir)

			actual := ir.Services[svcKey].Gce
			
			// If expected is nil, we expect nil OR empty Gce struct (depends on implementation)
			// Our implementation initializes Gce if it finds affinity.
			
			if tc.expectedGCE == nil {
				if actual != nil && actual.SessionAffinity != nil {
					t.Errorf("Expected nil SessionAffinity, got %v", actual.SessionAffinity)
				}
			} else {
				if diff := cmp.Diff(tc.expectedGCE, actual); diff != "" {
					t.Errorf("GCE Service IR mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
