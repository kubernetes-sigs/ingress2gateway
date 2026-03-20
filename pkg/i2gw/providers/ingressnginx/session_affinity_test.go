/*
Copyright The Kubernetes Authors.

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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestGCEFeature(t *testing.T) {
	testCases := []struct {
		name                    string
		ingress                 networkingv1.Ingress
		expectedSessionAffinity *emitterir.SessionAffinity
	}{
		{
			name: "No Affinity",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-affinity",
					Namespace: "default",
				},
			},
			expectedSessionAffinity: nil, // Should not modify service
		},
		{
			name: "Cookie Affinity",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cookie-affinity",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/affinity": "cookie",
					},
				},
			},
			expectedSessionAffinity: &emitterir.SessionAffinity{
				Metadata: emitterir.NewExtensionFeatureMetadata(
					"default/cookie-affinity",
					[]*field.Path{field.NewPath("default", "cookie-affinity", "metadata", "annotations", fmt.Sprintf("%q", "nginx.ingress.kubernetes.io/affinity"))},
					"Session affinity is not supported",
				),
				Type: "Cookie",
			},
		},
		{
			name: "Cookie Affinity with Expires",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cookie-affinity-expires",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/affinity":               "cookie",
						"nginx.ingress.kubernetes.io/session-cookie-expires": "3600",
					},
				},
			},
			expectedSessionAffinity: &emitterir.SessionAffinity{
				Metadata: emitterir.NewExtensionFeatureMetadata(
					"default/cookie-affinity-expires",
					[]*field.Path{
						field.NewPath("default", "cookie-affinity-expires", "metadata", "annotations", fmt.Sprintf("%q", "nginx.ingress.kubernetes.io/affinity")),
						field.NewPath("default", "cookie-affinity-expires", "metadata", "annotations", fmt.Sprintf("%q", "nginx.ingress.kubernetes.io/session-cookie-expires")),
					},
					"Session affinity is not supported",
				),
				Type:         "Cookie",
				CookieTTLSec: ptr.To(int64(3600)),
			},
		},
		{
			name: "Cookie Affinity with Name (unparsed - no emitter consumes it)",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cookie-affinity-name",
					Namespace: "default",
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/affinity":            "cookie",
						"nginx.ingress.kubernetes.io/session-cookie-name": "MY_COOKIE",
					},
				},
			},
			expectedSessionAffinity: &emitterir.SessionAffinity{
				Metadata: emitterir.NewExtensionFeatureMetadata(
					"default/cookie-affinity-name",
					[]*field.Path{field.NewPath("default", "cookie-affinity-name", "metadata", "annotations", fmt.Sprintf("%q", "nginx.ingress.kubernetes.io/affinity"))},
					"Session affinity is not supported",
				),
				Type: "Cookie",
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

			// Mock Route Logic (Simplified to match sessionAffinityFeature expectations)
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

			sessionAffinityFeature(func(_ notifications.MessageType, _ string, _ ...client.Object) {}, []networkingv1.Ingress{tc.ingress}, nil, &ir)

			actual := ir.Services[svcKey].SessionAffinity

			// If expected is nil, we expect nil OR empty SessionAffinity struct (depends on implementation)
			// Our implementation initializes SessionAffinity if it finds affinity.

			if tc.expectedSessionAffinity == nil {
				if actual != nil {
					t.Errorf("Expected nil SessionAffinity, got %v", actual)
				}
			} else {
				if diff := cmp.Diff(tc.expectedSessionAffinity, actual, cmp.AllowUnexported(emitterir.ExtensionFeatureMetadata{}, field.Path{})); diff != "" {
					t.Errorf("SessionAffinity IR mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
