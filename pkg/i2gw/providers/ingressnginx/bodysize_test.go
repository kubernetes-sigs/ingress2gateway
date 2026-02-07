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

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/logging"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestConvertNginxSizeToK8sQuantity(t *testing.T) {
	tests := []struct {
		name      string
		nginxSize string
		want      string
		wantErr   bool
	}{
		{
			name:      "lowercase b (bytes)",
			nginxSize: "1024b",
			want:      "1024",
			wantErr:   false,
		},
		{
			name:      "lowercase k stays as k",
			nginxSize: "100k",
			want:      "100k",
			wantErr:   false,
		},
		{
			name:      "lowercase m to K8s Mega",
			nginxSize: "10m",
			want:      "10M",
			wantErr:   false,
		},
		{
			name:      "lowercase g to K8s Giga",
			nginxSize: "5g",
			want:      "5G",
			wantErr:   false,
		},
		{
			name:      "no unit (bytes)",
			nginxSize: "512",
			want:      "512",
			wantErr:   false,
		},
		{
			name:      "whitespace trimmed",
			nginxSize: "  10m  ",
			want:      "10M",
			wantErr:   false,
		},
		{
			name:      "invalid format - letters only",
			nginxSize: "abc",
			want:      "",
			wantErr:   true,
		},
		{
			name:      "invalid unit - x",
			nginxSize: "10x",
			want:      "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertNginxSizeToK8sQuantity(tt.nginxSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertNginxSizeToK8sQuantity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("convertNginxSizeToK8sQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyBodySizeToEmitterIR_SetMaxSize(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	annotations := map[string]string{
		ProxyBodySizeAnnotation: "10m",
	}
	pIR, eIR := setupBodySizeTest(key, annotations)

	if err := applyBodySizeToEmitterIR(logging.Noop(), pIR, &eIR); err != nil {
		t.Fatalf("unexpected error applying body size: %v", err)
	}

	bodySizeIR := eIR.HTTPRoutes[key].BodySizeByRuleIdx[0]
	if bodySizeIR == nil {
		t.Fatalf("expected body size IR to be set for rule index 0")
	}
	if bodySizeIR.MaxSize.String() != "10M" {
		t.Fatalf("expected max size 10M, got %s", bodySizeIR.MaxSize.String())
	}
}

func TestApplyBodySizeToEmitterIR_SetBufferSize(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	annotations := map[string]string{
		ClientBodyBufferSizeAnnotation: "10m",
	}
	pIR, eIR := setupBodySizeTest(key, annotations)

	if err := applyBodySizeToEmitterIR(logging.Noop(), pIR, &eIR); err != nil {
		t.Fatalf("unexpected error applying body size: %v", err)
	}

	bodySizeIR := eIR.HTTPRoutes[key].BodySizeByRuleIdx[0]
	if bodySizeIR == nil {
		t.Fatalf("expected body size IR to be set for rule index 0")
	}
	if bodySizeIR.BufferSize.String() != "10M" {
		t.Fatalf("expected buffer size 10M, got %s", bodySizeIR.BufferSize.String())
	}
}

func TestApplyBodySizeToEmitterIR_SetMaxAndBufferSize(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	annotations := map[string]string{
		ProxyBodySizeAnnotation:        "100m",
		ClientBodyBufferSizeAnnotation: "50m",
	}
	pIR, eIR := setupBodySizeTest(key, annotations)

	if err := applyBodySizeToEmitterIR(logging.Noop(), pIR, &eIR); err != nil {
		t.Fatalf("unexpected error applying body size: %v", err)
	}

	bodySizeIR := eIR.HTTPRoutes[key].BodySizeByRuleIdx[0]
	if bodySizeIR == nil {
		t.Fatalf("expected body size IR to be set for rule index 0")
	}
	if bodySizeIR.MaxSize.String() != "100M" {
		t.Fatalf("expected max size 100M, got %s", bodySizeIR.MaxSize.String())
	}
	if bodySizeIR.BufferSize.String() != "50M" {
		t.Fatalf("expected buffer size 50M, got %s", bodySizeIR.BufferSize.String())
	}
}

func setupBodySizeTest(httpRouteKey types.NamespacedName, ingAnnotations map[string]string) (providerir.ProviderIR, emitterir.EmitterIR) {
	parentRefs := []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   httpRouteKey.Namespace,
			Name:        "ing",
			Annotations: ingAnnotations,
		},
		Spec: networkingv1.IngressSpec{},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: httpRouteKey.Namespace, Name: httpRouteKey.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: append([]gatewayv1.ParentReference(nil), parentRefs...),
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{},
			},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[httpRouteKey] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[httpRouteKey] = emitterir.HTTPRouteContext{HTTPRoute: route}
	return pIR, eIR
}
