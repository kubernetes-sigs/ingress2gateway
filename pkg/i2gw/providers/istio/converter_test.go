/*
Copyright 2023 The Kubernetes Authors.

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

package istio

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	istiov1beta1 "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_converter_convertGateway(t *testing.T) {
	type args struct {
		gw *istioclientv1beta1.Gateway
	}
	tests := []struct {
		name             string
		args             args
		wantGateway      *gatewayv1.Gateway
		wantAllowedHosts map[types.NamespacedName]map[string]sets.Set[string]
	}{
		{
			name: "gateway with TLS and hosts",
			args: args{
				gw: &istioclientv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "name",
						Namespace:   "test",
						Labels:      map[string]string{"k": "v"},
						Annotations: map[string]string{"k1": "v1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "object",
							},
						},
						Finalizers: []string{"finalizer1"},
					},
					Spec: istiov1beta1.Gateway{
						Servers: []*istiov1beta1.Server{
							{
								Name: "http",
								Port: &istiov1beta1.Port{
									Number:   80,
									Protocol: "HTTP",
								},
								Hosts: []string{
									"http/*.example.com",
									"http/*",
									"*/foo.example.com",
									"./foo.example.com",
									"*.example.com",
								},
							},
							{
								Name: "https",
								Port: &istiov1beta1.Port{
									Number:   443,
									Protocol: "HTTPS",
								},
								Tls: &istiov1beta1.ServerTLSSettings{
									Mode: istiov1beta1.ServerTLSSettings_PASSTHROUGH,
								},
								Hosts: []string{
									"https/*.example.com",
									"https/*",
									"*/foo.example.com",
									"./foo.example.com",
									"*.example.com",
								},
							},
							{
								Name: "http2",
								Port: &istiov1beta1.Port{
									Number:   443,
									Protocol: "HTTP2",
								},
								Tls: &istiov1beta1.ServerTLSSettings{
									Mode: istiov1beta1.ServerTLSSettings_SIMPLE,
								},
								Hosts: []string{
									"http2/*.example.com",
									"http2/*",
									"*/foo.example.com",
									"./foo.example.com",
									"*.example.com",
								},
							},
						},
					},
				},
			},
			wantGateway: &gatewayv1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: common.GatewayGVK.GroupVersion().String(),
					Kind:       common.GatewayGVK.Kind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "name",
					Namespace:   "test",
					Labels:      map[string]string{"k": "v"},
					Annotations: map[string]string{"k1": "v1"},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "object",
						},
					},
					Finalizers: []string{"finalizer1"},
				},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: K8SGatewayClassName,
					Listeners: []gatewayv1.Listener{
						{
							Name:     "http-protocol-http-ns-wildcard.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("*.example.com")),
							Port:     80,
							Protocol: "HTTP",
						},
						{
							Name:     "http-protocol-http-ns-wildcard",
							Port:     80,
							Protocol: "HTTP",
						},
						{
							Name:     "http-protocol-wildcard-ns-foo.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("foo.example.com")),
							Port:     80,
							Protocol: "HTTP",
						},
						{
							Name:     "http-protocol-dot-ns-foo.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("foo.example.com")),
							Port:     80,
							Protocol: "HTTP",
						},
						{
							Name:     "http-protocol-wildcard-ns-wildcard.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("*.example.com")),
							Port:     80,
							Protocol: "HTTP",
						},
						{
							Name:     "https-protocol-https-ns-wildcard.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("*.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Passthrough"))},
						},
						{
							Name:     "https-protocol-https-ns-wildcard",
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Passthrough"))},
						},
						{
							Name:     "https-protocol-wildcard-ns-foo.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("foo.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Passthrough"))},
						},
						{
							Name:     "https-protocol-dot-ns-foo.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("foo.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Passthrough"))},
						},
						{
							Name:     "https-protocol-wildcard-ns-wildcard.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("*.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Passthrough"))},
						},
						{
							Name:     "https-protocol-http2-ns-wildcard.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("*.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Terminate"))},
						},
						{
							Name:     "https-protocol-http2-ns-wildcard",
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Terminate"))},
						},
						{
							Name:     "https-protocol-wildcard-ns-foo.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("foo.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Terminate"))},
						},
						{
							Name:     "https-protocol-dot-ns-foo.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("foo.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Terminate"))},
						},
						{
							Name:     "https-protocol-wildcard-ns-wildcard.example.com",
							Hostname: common.PtrTo(gatewayv1.Hostname("*.example.com")),
							Port:     443,
							Protocol: "HTTPS",
							TLS:      &gatewayv1.GatewayTLSConfig{Mode: common.PtrTo(gatewayv1.TLSModeType("Terminate"))},
						},
					},
				},
			},
			wantAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
				{
					Namespace: "test",
					Name:      "name",
				}: {
					"*":     sets.New[string]("*.example.com", "foo.example.com"),
					".":     sets.New[string]("foo.example.com"),
					"http":  sets.New[string]("*.example.com", "*"),
					"http2": sets.New[string]("*.example.com", "*"),
					"https": sets.New[string]("*.example.com", "*"),
				},
			},
		},
		{
			name: "nil port -> gw with no listeners",
			args: args{
				gw: &istioclientv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "test",
					},
				},
			},
			wantGateway: &gatewayv1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: common.GatewayGVK.GroupVersion().String(),
					Kind:       common.GatewayGVK.Kind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "test",
				},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: K8SGatewayClassName,
				},
			},
			wantAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
				{
					Namespace: "test",
					Name:      "name",
				}: {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newConverter()
			if got := c.convertGateway(tt.args.gw, field.NewPath("")); !apiequality.Semantic.DeepEqual(got, tt.wantGateway) {
				t.Errorf("converter.convertGateway() = %+v, want %+v, diff (-want +got): %s", got, tt.wantGateway, cmp.Diff(tt.wantGateway, got))
			}

			if got := c.gwAllowedHosts; !apiequality.Semantic.DeepEqual(got, tt.wantAllowedHosts) {
				t.Errorf("incorrectly parsed gwAllowedHosts, got: %+v, want %+v, diff (-want +got): %s", got, tt.wantAllowedHosts, cmp.Diff(tt.wantAllowedHosts, got))
			}
		})
	}
}
