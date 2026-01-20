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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestRewriteFeature(t *testing.T) {
	regexType := gatewayv1.PathMatchRegularExpression
	prefixType := gatewayv1.PathMatchPathPrefix
	replacePrefix := "/"
	replaceFull := "/new-path"

	testCases := []struct {
		name        string
		ingress     networkingv1.Ingress
		matchValue  string
		matchType   gatewayv1.PathMatchType
		expected    []gatewayv1.HTTPRouteFilter
		expectMatch gatewayv1.PathMatchType // To check if we reverted to Prefix
		expectValue string
	}{
		{
			name: "Prefix Strip: /foo(/|$)(.*) -> /$2",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
						"nginx.ingress.kubernetes.io/use-regex":      "true",
					},
				},
			},
			matchValue: "/foo(/|$)(.*)",
			matchType:  regexType,
			expected: []gatewayv1.HTTPRouteFilter{
				{
					Type: gatewayv1.HTTPRouteFilterURLRewrite,
					URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
						Path: &gatewayv1.HTTPPathModifier{
							Type:               gatewayv1.PrefixMatchHTTPPathModifier,
							ReplacePrefixMatch: &replacePrefix,
						},
					},
				},
			},
			expectMatch: prefixType, // upgraded to prefix
			expectValue: "/foo",
		},
		{
			name: "Static Rewrite: /old -> /new-path",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/rewrite-target": "/new-path",
					},
				},
			},
			matchValue: "/old",
			matchType:  prefixType,
			expected: []gatewayv1.HTTPRouteFilter{
				{
					Type: gatewayv1.HTTPRouteFilterURLRewrite,
					URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
						Path: &gatewayv1.HTTPPathModifier{
							Type:            gatewayv1.FullPathHTTPPathModifier,
							ReplaceFullPath: &replaceFull,
						},
					},
				},
			},
			expectMatch: prefixType,
			expectValue: "/old",
		},
		{
			name: "Unsupported Complex Regex",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/rewrite-target": "/$1/$2",
					},
				},
			},
			matchValue: "/foo/(.*)/(.*)",
			matchType:  regexType,
			expected:   nil, // No filter added
			expectMatch: regexType,
			expectValue: "/foo/(.*)/(.*)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			key := types.NamespacedName{Namespace: "default", Name: "test"}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  &tc.matchType,
										Value: ptr.To(tc.matchValue),
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

			rewriteFeature([]networkingv1.Ingress{tc.ingress}, nil, &ir)

			filters := ir.HTTPRoutes[key].HTTPRoute.Spec.Rules[0].Filters
			if diff := cmp.Diff(tc.expected, filters); diff != "" {
				t.Errorf("Filters mismatch (-want +got):\n%s", diff)
			}
			
			match :=ir.HTTPRoutes[key].HTTPRoute.Spec.Rules[0].Matches[0]
			if *match.Path.Type != tc.expectMatch {
				t.Errorf("MatchType mismatch: want %v, got %v", tc.expectMatch, *match.Path.Type)
			}
			if *match.Path.Value != tc.expectValue {
				t.Errorf("MatchValue mismatch: want %v, got %v", tc.expectValue, *match.Path.Value)
			}
		})
	}
}
