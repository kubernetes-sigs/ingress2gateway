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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestConvertNginxSizeToK8sQuantity(t *testing.T) {
	t.Parallel()
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
			name:      "k becomes Ki",
			nginxSize: "100k",
			want:      "100Ki",
			wantErr:   false,
		},
		{
			name:      "K becomes Ki, (testing capitals)",
			nginxSize: "100K",
			want:      "100Ki",
			wantErr:   false,
		},
		{
			name:      "m becomes Mi",
			nginxSize: "50m",
			want:      "50Mi",
			wantErr:   false,
		},
		{
			name:      "M becomes Mi, (testing capitals)",
			nginxSize: "50M",
			want:      "50Mi",
			wantErr:   false,
		},
		{
			name:      "g becomes Gi",
			nginxSize: "128g",
			want:      "128Gi",
			wantErr:   false,
		},
		{
			name:      "G becomes Gi, (testing capitals)",
			nginxSize: "128G",
			want:      "128Gi",
			wantErr:   false,
		},
		{
			name:      "obscure the unit",
			nginxSize: "256",
			want:      "256",
			wantErr:   false,
		},
		{
			name:      "trim whitespace",
			nginxSize: "  50g   ",
			want:      "50Gi",
			wantErr:   false,
		},
		{
			name:      "invalid number",
			nginxSize: "abc",
			wantErr:   true,
		},
		{
			name:      "invalid unit",
			nginxSize: "10x",
			wantErr:   true,
		},
		{
			name:      "empty value",
			nginxSize: "",
			wantErr:   true,
		},
		{
			name:      "zero bytes, without a unit",
			nginxSize: "0",
			want:      "0",
			wantErr:   false,
		},
		{
			name:      "zero bytes, with a unit",
			nginxSize: "0m",
			want:      "0Mi",
			wantErr:   false,
		},
		{
			name:      "invalid number format, reject decimal",
			nginxSize: "1.5m",
			wantErr:   true,
		},
		{
			name:      "invalid number format, reject negative",
			nginxSize: "-1m",
			wantErr:   true,
		},
		{
			name:      "whitespace only",
			nginxSize: "    ",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func TestParseClientMaxBodySize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		nginxSize string
		want      string
		wantErr   bool
	}{
		{
			name:      "no unit",
			nginxSize: "10",
			want:      "10",
			wantErr:   false,
		},
		{
			name:      "bytes",
			nginxSize: "10b",
			want:      "10",
			wantErr:   false,
		},
		{
			name:      "kilobytes",
			nginxSize: "10k",
			want:      "10Ki",
			wantErr:   false,
		},
		{
			name:      "megabytes",
			nginxSize: "10m",
			want:      "10Mi",
			wantErr:   false,
		},
		{
			name:      "gigabytes",
			nginxSize: "10g",
			want:      "10Gi",
			wantErr:   false,
		},
		{
			name:      "surrounding whitespace",
			nginxSize: "   10m   ",
			want:      "10Mi",
			wantErr:   false,
		},
		{
			name:      "invalid unit",
			nginxSize: "10x",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseClientMaxBodySize(tt.nginxSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseClientMaxBodySize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if got != nil {
					t.Errorf("expected nil quantity, got %v", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("expected non-nil quantity")
			}

			if got.String() != tt.want {
				t.Errorf("parseClientMaxBodySize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIngressForRule(t *testing.T) {
	t.Parallel()
	ingress := &networkingv1.Ingress{}

	tests := []struct {
		name    string
		sources []providerir.BackendSource
		want    *networkingv1.Ingress
	}{
		{
			name: "finds ingress source",
			sources: []providerir.BackendSource{
				{Ingress: ingress},
			},
			want: ingress,
		},
		{
			name: "does not find ingress source",
			sources: []providerir.BackendSource{
				{Ingress: nil},
			},
			want: nil,
		},
		{
			name:    "empty sources",
			sources: nil,
			want:    nil,
		},
		{
			name: "return firtst ingress source",
			sources: []providerir.BackendSource{
				{Ingress: nil},
				{Ingress: ingress},
			},
			want: ingress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ingressForRule(tt.sources)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBodySizeFeature(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		annotation string
		wantSize   string
		wantNil    bool
	}{
		{
			name:       "sets body size by rule",
			annotation: "10m",
			wantSize:   "10Mi",
			wantNil:    false,
		},
		{
			name:       "missing annotation does not set body size",
			annotation: "",
			wantSize:   "",
			wantNil:    true,
		},
		{
			name:       "Whitespace only annotation",
			annotation: "    ",
			wantNil:    true,
		},
		{
			name:       "invalid annotation",
			annotation: "10x",
			wantSize:   "",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ingress := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: "default",
					Annotations: map[string]string{
						nginxClientMaxBodySizeAnnotation: tt.annotation,
					},
				},
			}

			key := types.NamespacedName{
				Namespace: ingress.Namespace,
				Name:      "route",
			}
			ir := providerir.ProviderIR{
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					key: {
						HTTPRoute: gatewayv1.HTTPRoute{
							Spec: gatewayv1.HTTPRouteSpec{
								Rules: []gatewayv1.HTTPRouteRule{{}},
							},
						},
						RuleBackendSources: [][]providerir.BackendSource{
							{{Ingress: &ingress}},
						},
					},
				},
			}

			BodySizeFeature(notifications.NoopNotify, nil, nil, &ir)

			got := ir.HTTPRoutes[key].ProviderSpecificIR.BodySizeByRuleIdx[0]
			if (got == nil) != tt.wantNil {
				t.Fatal("expected body size for rule 0")
			}
			if tt.wantNil {
				return
			}
			if got.MaxSize.String() != tt.wantSize {
				t.Errorf("got %q, want %q", got.MaxSize.String(), tt.wantSize)
			}
		})
	}
}

func TestProcessBodySizeAnnotation(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example",
			Namespace: "default",
		},
	}

	tests := []struct {
		name     string
		bodySize string
		wantSize string
		wantNil  bool
	}{
		{
			name:     "valid body size",
			bodySize: "10m",
			wantSize: "10Mi",
			wantNil:  false,
		},
		{
			name:     "invalid body size",
			bodySize: "10x",
			wantSize: "",
			wantNil:  true,
		},
		{
			name:     "missing body size returns nil",
			bodySize: "",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processBodySizeAnnotation(
				notifications.NoopNotify,
				ingress,
				tt.bodySize,
			)
			if (got == nil) != tt.wantNil {
				t.Fatalf("got nil = %v, wanted nil =%v", got == nil, tt.wantNil)
			}
			if !tt.wantNil && got.MaxSize.String() != tt.wantSize {
				t.Errorf("got %q, want %q", got.MaxSize.String(), tt.wantSize)
			}
		})
	}
}
