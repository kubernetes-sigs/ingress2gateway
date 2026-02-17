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
	"slices"
	"testing"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestApplyIPRangeControlToEmitterIR(t *testing.T) {
	testCases := []struct {
		name              string
		ingress           networkingv1.Ingress
		expectedAllowList []string
		expectedDenyList  []string
	}{
		{
			name: "whitelist only - single CIDR",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ip-range-defaults",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/whitelist-source-range": "192.168.1.0/24",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedAllowList: []string{"192.168.1.0/24"},
			expectedDenyList:  nil,
		},
		{
			name: "whitelist only - multiple CIDRs",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ip-range-defaults",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/whitelist-source-range": "192.168.1.0/24,10.0.0.0/8,172.16.0.0/12",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedAllowList: []string{"192.168.1.0/24", "10.0.0.0/8", "172.16.0.0/12"},
			expectedDenyList:  nil,
		},
		{
			name: "whitelist with whitespace trimming",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ip-range-defaults",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/whitelist-source-range": " 192.168.1.0/24 , 10.0.0.0/8 ",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedAllowList: []string{"192.168.1.0/24", "10.0.0.0/8"},
			expectedDenyList:  nil,
		},
		{
			name: "denylist only - single CIDR",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ip-range-defaults",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/denylist-source-range": "203.0.113.0/24",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedAllowList: nil,
			expectedDenyList:  []string{"203.0.113.0/24"},
		},
		{
			name: "denylist only - multiple CIDRs",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ip-range-defaults",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/denylist-source-range": "203.0.113.0/24,198.51.100.0/24",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedAllowList: nil,
			expectedDenyList:  []string{"203.0.113.0/24", "198.51.100.0/24"},
		},
		{
			name: "both whitelist and denylist",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ip-range-defaults",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/whitelist-source-range": "192.168.1.0/24,10.0.0.0/8",
						"nginx.ingress.kubernetes.io/denylist-source-range":  "203.0.113.0/24",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedAllowList: []string{"192.168.1.0/24", "10.0.0.0/8"},
			expectedDenyList:  []string{"203.0.113.0/24"},
		},
		{
			name: "no IP range annotations",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ip-range-defaults",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "example.com"}},
				},
			},
			expectedAllowList: nil,
			expectedDenyList:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pIR := providerir.ProviderIR{
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}
			eIR := emitterir.EmitterIR{
				HTTPRoutes: make(map[types.NamespacedName]emitterir.HTTPRouteContext),
			}

			key := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: common.RouteName(tc.ingress.Name, "example.com")}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: tc.ingress.Namespace, Name: key.Name},
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: ptr.To(gatewayv1.PathMatchPathPrefix), Value: ptr.To("/")}}},
						},
					},
				},
			}

			// Provider IR setup (for sources)
			pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
				HTTPRoute: route,
				RuleBackendSources: [][]providerir.BackendSource{{
					{Ingress: &tc.ingress},
				}},
			}

			// Emitter IR setup (target)
			eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{
				HTTPRoute: route,
			}

			applyIPRangeControlToEmitterIR(pIR, &eIR)

			result := eIR.HTTPRoutes[key]
			var ipRangeControl *emitterir.IPRangeControl
			if result.IPRangeControlByRuleIdx != nil {
				ipRangeControl = result.IPRangeControlByRuleIdx[0]
			}

			if tc.expectedAllowList == nil && tc.expectedDenyList == nil {
				if ipRangeControl != nil {
					t.Fatalf("Expected nil IPRangeControl, got %v", ipRangeControl)
				}
				return
			}
			if ipRangeControl == nil {
				t.Fatalf("Expected IPRangeControl to be set, got nil")
			}

			if !slices.Equal(ipRangeControl.AllowList, tc.expectedAllowList) {
				t.Errorf("AllowList mismatch: expected %v, got %v", tc.expectedAllowList, ipRangeControl.AllowList)
			}
			if !slices.Equal(ipRangeControl.DenyList, tc.expectedDenyList) {
				t.Errorf("DenyList mismatch: expected %v, got %v", tc.expectedDenyList, ipRangeControl.DenyList)
			}
		})
	}
}
