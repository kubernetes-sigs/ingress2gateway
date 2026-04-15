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

package traefik

import (
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_unsupportedAnnotationsFeature(t *testing.T) {
	testCases := []struct {
		name             string
		annotations      map[string]string
		expectedWarnings int
	}{
		{
			name:             "unknown traefik annotation emits warning",
			annotations:      map[string]string{"traefik.ingress.kubernetes.io/router.unknown": "value"},
			expectedWarnings: 1,
		},
		{
			name: "multiple unknown traefik annotations emit warnings",
			annotations: map[string]string{
				"traefik.ingress.kubernetes.io/router.middlewares": "default-auth@kubernetescrd",
				"traefik.ingress.kubernetes.io/router.priority":    "5",
			},
			expectedWarnings: 2,
		},
		{
			name:             "supported annotations -- no warnings",
			annotations:      map[string]string{RouterTLSAnnotation: "true", RouterEntrypointsAnnotation: "websecure"},
			expectedWarnings: 0,
		},
		{
			name:             "no annotations -- no warnings",
			annotations:      map[string]string{},
			expectedWarnings: 0,
		},
		{
			name:             "unrelated annotations -- no warnings",
			annotations:      map[string]string{"some.other/annotation": "value"},
			expectedWarnings: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingress := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "my-app",
					Namespace:   "default",
					Annotations: tc.annotations,
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(nil, "my-app")}},
				},
			}

			var warnings int
			notify := func(mt notifications.MessageType, _ string, _ ...client.Object) {
				if mt == notifications.WarningNotification {
					warnings++
				}
			}

			errs := unsupportedAnnotationsFeature(notify, []networkingv1.Ingress{ingress}, nil, &providerir.ProviderIR{})
			if len(errs) != 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
			if warnings != tc.expectedWarnings {
				t.Errorf("expected %d warnings, got %d", tc.expectedWarnings, warnings)
			}
		})
	}
}
