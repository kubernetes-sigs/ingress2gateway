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

	common_emitter "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/common_emitter"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestTimeoutFeature(t *testing.T) {
	testCases := []struct {
		name        string
		annotations map[string]string
		wantBackend *gatewayv1.Duration
		wantErr     bool
	}{
		{
			name:        "no timeouts",
			annotations: map[string]string{},
			wantBackend: nil,
		},
		{
			name: "seconds parse + multiplier",
			annotations: map[string]string{
				ProxyReadTimeoutAnnotation: "2",
			},
			wantBackend: common.PtrTo[gatewayv1.Duration]("20s"),
		},
		{
			name: "max + multiplier",
			annotations: map[string]string{
				ProxyConnectTimeoutAnnotation: "1",
				ProxySendTimeoutAnnotation:    "2",
				ProxyReadTimeoutAnnotation:    "3",
			},
			wantBackend: common.PtrTo[gatewayv1.Duration]("30s"),
		},
		{
			name: "rejects duration strings",
			annotations: map[string]string{
				ProxyReadTimeoutAnnotation: "1s",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ing := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Namespace:   "default",
					Annotations: tc.annotations,
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend:  networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "svc", Port: networkingv1.ServiceBackendPort{Number: 80}}},
							}}},
						},
					}},
				},
			}

			ir := providerir.ProviderIR{HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext)}
			key := types.NamespacedName{Namespace: ing.Namespace, Name: common.RouteName(ing.Name, "example.com")}
			route := gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: ing.Namespace, Name: key.Name},
				Spec:       gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{{}}},
			}
			ir.HTTPRoutes[key] = providerir.HTTPRouteContext{
				HTTPRoute: route,
				RuleBackendSources: [][]providerir.BackendSource{{
					{Ingress: &ing},
				}},
			}

			eir := providerir.ToEmitterIR(ir)

			errs := timeoutFeature([]networkingv1.Ingress{ing}, nil, &ir, &eir)
			if tc.wantErr {
				if len(errs) == 0 {
					t.Fatalf("expected error")
				}
				return
			}
			if len(errs) > 0 {
				t.Fatalf("expected no errors, got %v", errs)
			}

			// Timeout feature should populate IR, not mutate the HTTPRoute directly.
			if got := ir.HTTPRoutes[key].HTTPRoute.Spec.Rules[0].Timeouts; got != nil {
				t.Fatalf("expected no direct HTTPRoute mutation, got timeouts %v", got)
			}

			// Apply common emitter (maps EmitterIR fields onto core Gateway API).
			commonEmitter := common_emitter.NewEmitter()
			eir, errs = commonEmitter.Emit(eir)
			if len(errs) > 0 {
				t.Fatalf("expected no common emitter errors, got %v", errs)
			}

			hctx, ok := eir.HTTPRoutes[key]
			if !ok {
				t.Fatalf("expected HTTPRoute in EmitterIR")
			}

			got := hctx.Spec.Rules[0].Timeouts
			if tc.wantBackend == nil {
				if got != nil && got.Request != nil {
					t.Fatalf("expected no request timeout, got %v", *got.Request)
				}
				return
			}
			if got == nil || got.Request == nil {
				t.Fatalf("expected request timeout to be set")
			}
			if *got.Request != *tc.wantBackend {
				t.Fatalf("expected %v, got %v", *tc.wantBackend, *got.Request)
			}
		})
	}
}
