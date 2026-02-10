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

func TestAffinityFeature(t *testing.T) {
	testCases := []struct {
		name      string
		ingresses []networkingv1.Ingress
		pIR       *providerir.ProviderIR
		expected  *providerir.ProviderIR
	}{
		{
			name: "valid affinity with max-age",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ing",
						Namespace: "default",
						Annotations: map[string]string{
							AffinityAnnotation:            "cookie",
							SessionCookieMaxAgeAnnotation: "172800",
						},
					},
				},
			},
			pIR: &providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "test-route"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							Spec: gatewayv1.HTTPRouteSpec{
								Rules: []gatewayv1.HTTPRouteRule{
									{
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "test-svc",
													},
												},
											},
										},
									},
								},
							},
						},
						RuleBackendSources: [][]providerir.BackendSource{
							{
								{
									Ingress: &networkingv1.Ingress{
										ObjectMeta: metav1.ObjectMeta{
											Name:      "test-ing",
											Namespace: "default",
											Annotations: map[string]string{
												AffinityAnnotation:            "cookie",
												SessionCookieMaxAgeAnnotation: "172800",
											},
										},
									},
								},
							},
						},
					},
				},
				Services: map[types.NamespacedName]providerir.ProviderSpecificServiceIR{},
			},
			expected: &providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					{Namespace: "default", Name: "test-route"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							Spec: gatewayv1.HTTPRouteSpec{
								Rules: []gatewayv1.HTTPRouteRule{
									{
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: "test-svc",
													},
												},
											},
										},
									},
								},
							},
						},
						RuleBackendSources: [][]providerir.BackendSource{
							{
								{
									Ingress: &networkingv1.Ingress{
										ObjectMeta: metav1.ObjectMeta{
											Name:      "test-ing",
											Namespace: "default",
											Annotations: map[string]string{
												AffinityAnnotation:            "cookie",
												SessionCookieMaxAgeAnnotation: "172800",
											},
										},
									},
								},
							},
						},
					},
				},
				Services: map[types.NamespacedName]providerir.ProviderSpecificServiceIR{
					{Namespace: "default", Name: "test-svc"}: {
						Gce: &gce.ServiceIR{
							SessionAffinity: &gce.SessionAffinityConfig{
								AffinityType: "GENERATED_COOKIE",
								CookieTTLSec: ptr.To[int64](172800),
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := affinityFeature(tc.ingresses, nil, tc.pIR)
			if len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if diff := cmp.Diff(tc.expected, tc.pIR); diff != "" {
				t.Errorf("affinityFeature() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
