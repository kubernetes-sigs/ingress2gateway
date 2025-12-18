/*
Copyright 2025 The Kubernetes Authors.

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

package annotations

import (
	"testing"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWebSocketServicesFeature(t *testing.T) {
	t.Run("with annotation", func(t *testing.T) {
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "websocket-ingress",
				Namespace: "default",
				Annotations: map[string]string{
					nginxWebSocketServicesAnnotation: "websocket-service",
				},
			},
		}

		ir := providerir.ProviderIR{}
		errs := WebSocketServicesFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
		if len(errs) > 0 {
			t.Errorf("Unexpected errors: %v", errs)
		}
	})

	t.Run("without annotation", func(t *testing.T) {
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "regular-ingress",
				Namespace: "default",
			},
		}

		ir := providerir.ProviderIR{}
		errs := WebSocketServicesFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
		if len(errs) > 0 {
			t.Errorf("Unexpected errors: %v", errs)
		}
	})
}
