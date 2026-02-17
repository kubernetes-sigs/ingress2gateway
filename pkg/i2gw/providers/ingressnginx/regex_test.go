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
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestRegexFeature(t *testing.T) {
	regexType := gatewayv1.PathMatchRegularExpression
	prefixType := gatewayv1.PathMatchPathPrefix

	testCases := []struct {
		name     string
		ingress  networkingv1.Ingress
		expected []gatewayv1.HTTPRouteMatch
	}{
		{
			name: "Should map to RegularExpression when use-regex annotation is true",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "regex-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/use-regex": "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/users/.*/profile",
											PathType: ptr.To(networkingv1.PathTypeImplementationSpecific),
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "service1",
													Port: networkingv1.ServiceBackendPort{Number: 80},
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
			expected: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &regexType,
						Value: ptr.To("/users/.*/profile.*"),
					},
				},
			},
		},
		{
			name: "Should default to PathPrefix when use-regex is missing",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prefix-ingress",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/api",
											PathType: ptr.To(networkingv1.PathTypePrefix),
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "service1",
													Port: networkingv1.ServiceBackendPort{Number: 80},
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
			expected: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &prefixType,
						Value: ptr.To("/api"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			// Manual IR setup simulating what common.ToIR would produce
			key := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: common.RouteName(tc.ingress.Name, "example.com")}

			// Determine initial match type based on input PathType (simulating generic conversion)
			var initialMatchType *gatewayv1.PathMatchType
			if *tc.ingress.Spec.Rules[0].HTTP.Paths[0].PathType == networkingv1.PathTypePrefix {
				initialMatchType = &prefixType
			} else {
				// ImplementationSpecific often defaults to Prefix if not handled, or just stays nil/impl-specific
				// For the sake of this test, let's assume common.ToIR set it to something or we are testing the overwrite.
				// But common.ToIR throws error for ImplSpecific if no custom converter.
				// However, regexFeature runs AFTER common.ToIR.
				// Let's assume common.ToIR generated a Prefix match (soft default) or checks if feature handles it.
				// Actually, simplified: we just want to see if regexFeature *updates* it.
				// So we init with Prefix for both cases.
				initialMatchType = &prefixType
			}

			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tc.ingress.Namespace,
					Name:      key.Name,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{"example.com"},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  initialMatchType,
										Value: ptr.To(tc.ingress.Spec.Rules[0].HTTP.Paths[0].Path),
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

			regexFeature([]networkingv1.Ingress{tc.ingress}, nil, &ir)

			// We expect only 1 route
			if len(ir.HTTPRoutes) != 1 {
				t.Fatalf("Expected 1 HTTPRoute, got %d", len(ir.HTTPRoutes))
			}

			for _, routeCtx := range ir.HTTPRoutes {
				if len(routeCtx.Spec.Rules) != 1 {
					t.Fatalf("Expected 1 Rule, got %d", len(routeCtx.Spec.Rules))
				}
				if diff := cmp.Diff(tc.expected, routeCtx.Spec.Rules[0].Matches); diff != "" {
					t.Errorf("Unexpected matches diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
