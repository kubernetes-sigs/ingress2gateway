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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"google.golang.org/protobuf/types/known/durationpb"
	istiov1beta1 "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func Test_resourcesToIRConverter_convertGateway(t *testing.T) {
	type args struct {
		gw *istioclientv1beta1.Gateway
	}
	tests := []struct {
		name             string
		args             args
		wantGateway      *gatewayv1.Gateway
		wantAllowedHosts map[types.NamespacedName]map[string]sets.Set[string]
		wantError        bool
	}{
		{
			name: "gateway with TLS and hosts",
			args: args{
				gw: &istioclientv1beta1.Gateway{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Gateway",
						APIVersion: "networking.istio.io/v1",
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
					TypeMeta: metav1.TypeMeta{
						Kind:       "Gateway",
						APIVersion: "networking.istio.io/v1",
					},
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
		{
			name: "unknown istio server protocol returns an error",
			args: args{
				gw: &istioclientv1beta1.Gateway{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Gateway",
						APIVersion: "networking.istio.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "test",
					},
					Spec: istiov1beta1.Gateway{
						Servers: []*istiov1beta1.Server{
							{
								Name: "http",
								Port: &istiov1beta1.Port{
									Number:   80,
									Protocol: "HTTP-TEST",
								},
								Hosts: []string{
									"http/*.example.com",
									"http/*",
									"*/foo.example.com",
									"./foo.example.com",
									"*.example.com",
								},
							},
						},
					},
				},
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newResourcesToIRConverter()
			got, errList := c.convertGateway(tt.args.gw, field.NewPath(""))
			if tt.wantError && len(errList) == 0 {
				t.Errorf("resourcesToIRConverter.convertGateway().errList = %+v, wantError %+v", errList, tt.wantError)
			}

			if !apiequality.Semantic.DeepEqual(got, tt.wantGateway) {
				t.Errorf("resourcesToIRConverter.convertGateway().gateway = %+v, want %+v, diff (-want +got): %s", got, tt.wantGateway, cmp.Diff(tt.wantGateway, got))
			}

			if got := c.gwAllowedHosts; !apiequality.Semantic.DeepEqual(got, tt.wantAllowedHosts) {
				t.Errorf("incorrectly parsed gwAllowedHosts, got: %+v, want %+v, diff (-want +got): %s", got, tt.wantAllowedHosts, cmp.Diff(tt.wantAllowedHosts, got))
			}
		})
	}
}

func Test_resourcesToIRConverter_convertVsHTTPRoutes(t *testing.T) {
	type args struct {
		virtualService   *istioclientv1beta1.VirtualService
		istioHTTPRoutes  []*istiov1beta1.HTTPRoute
		allowedHostnames []string
	}
	tests := []struct {
		name      string
		args      args
		want      []*gatewayv1.HTTPRoute
		wantError bool
	}{
		{
			name: "objectMeta field is converted and hosts are set",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Namespace:   "ns",
						Labels:      map[string]string{"k": "v"},
						Annotations: map[string]string{"k1": "v1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "object",
							},
						},
						Finalizers: []string{"finalizer1"},
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{},
				},
				allowedHostnames: []string{"*.test.com", "foo.bar"},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-idx-0",
						Namespace:   "ns",
						Labels:      map[string]string{"k": "v"},
						Annotations: map[string]string{"k1": "v1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "object",
							},
						},
						Finalizers: []string{"finalizer1"},
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules:     []gatewayv1.HTTPRouteRule{{}},
						Hostnames: []gatewayv1.Hostname{"*.test.com", "foo.bar"},
					},
				},
			},
		},
		{
			name: "backendRefs are generated",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Name: "route-name",
						Route: []*istiov1beta1.HTTPRouteDestination{
							{
								Destination: &istiov1beta1.Destination{
									Host: "reviews.prod.svc.cluster.local",
									Port: &istiov1beta1.PortSelector{
										Number: 5555,
									},
								},
								Weight: 50,
							},
							{
								Destination: &istiov1beta1.Destination{
									Host: "reviews",
									Port: &istiov1beta1.PortSelector{
										Number: 6555,
									},
								},
								Weight: 50,
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-route-name",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								BackendRefs: []gatewayv1.HTTPBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name:      "reviews",
												Namespace: common.PtrTo[gatewayv1.Namespace]("prod"),
												Port:      common.PtrTo[gatewayv1.PortNumber](5555),
											},
											Weight: common.PtrTo[int32](50),
										},
									},
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name:      "reviews",
												Namespace: common.PtrTo[gatewayv1.Namespace]("ns"),
												Port:      common.PtrTo[gatewayv1.PortNumber](6555),
											},
											Weight: common.PtrTo[int32](50),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "match.Uri is converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								Name: "match-uri-exact",
								Uri: &istiov1beta1.StringMatch{
									MatchType: &istiov1beta1.StringMatch_Exact{
										Exact: "v1",
									},
								},
							},
							{
								Name: "match-uri-prefix",
								Uri: &istiov1beta1.StringMatch{
									MatchType: &istiov1beta1.StringMatch_Prefix{
										Prefix: "v2",
									},
								},
							},
							{
								Name: "match-uri-regex",
								Uri: &istiov1beta1.StringMatch{
									MatchType: &istiov1beta1.StringMatch_Regex{
										Regex: `([A-Z])*\w+`,
									},
								},
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo[gatewayv1.PathMatchType](gatewayv1.PathMatchExact),
											Value: common.PtrTo[string]("v1"),
										},
									},
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo[gatewayv1.PathMatchType](gatewayv1.PathMatchPathPrefix),
											Value: common.PtrTo[string]("v2"),
										},
									},
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo[gatewayv1.PathMatchType](gatewayv1.PathMatchRegularExpression),
											Value: common.PtrTo[string](`([A-Z])*\w+`),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "match.Headers are converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								Headers: map[string]*istiov1beta1.StringMatch{
									"header1": {
										MatchType: &istiov1beta1.StringMatch_Exact{
											Exact: "v1",
										},
									},
								},
							},
							{
								Headers: map[string]*istiov1beta1.StringMatch{
									"header2": {
										MatchType: &istiov1beta1.StringMatch_Regex{
											Regex: "v1[A-Z]",
										},
									},
								},
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Headers: []gatewayv1.HTTPHeaderMatch{
											{
												Type:  common.PtrTo[gatewayv1.HeaderMatchType](gatewayv1.HeaderMatchExact),
												Name:  "header1",
												Value: "v1",
											},
										},
									},
									{
										Headers: []gatewayv1.HTTPHeaderMatch{
											{
												Type:  common.PtrTo[gatewayv1.HeaderMatchType](gatewayv1.HeaderMatchRegularExpression),
												Name:  "header2",
												Value: "v1[A-Z]",
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
		{
			name: "match.QueryParams are converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								QueryParams: map[string]*istiov1beta1.StringMatch{
									"q1": {
										MatchType: &istiov1beta1.StringMatch_Exact{
											Exact: "v1",
										},
									},
								},
							},
						},
					},
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								QueryParams: map[string]*istiov1beta1.StringMatch{
									"q2": {
										MatchType: &istiov1beta1.StringMatch_Regex{
											Regex: "q1[A-Z]",
										},
									},
								},
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										QueryParams: []gatewayv1.HTTPQueryParamMatch{
											{
												Type:  common.PtrTo[gatewayv1.QueryParamMatchType](gatewayv1.QueryParamMatchExact),
												Name:  "q1",
												Value: "v1",
											},
										},
									},
								},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-1",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										QueryParams: []gatewayv1.HTTPQueryParamMatch{
											{
												Type:  common.PtrTo[gatewayv1.QueryParamMatchType](gatewayv1.QueryParamMatchRegularExpression),
												Name:  "q2",
												Value: "q1[A-Z]",
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
		{
			name: "match.Method is converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								Method: &istiov1beta1.StringMatch{
									MatchType: &istiov1beta1.StringMatch_Exact{
										Exact: "GET",
									},
								},
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Method: common.PtrTo[gatewayv1.HTTPMethod]("GET"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "route.Redirect is converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Redirect: &istiov1beta1.HTTPRedirect{
							RedirectCode: 302,
							Uri:          "redirect-uri",
							RedirectPort: &istiov1beta1.HTTPRedirect_Port{
								Port: 8080,
							},
							Scheme: "http",
						},
					},
					{
						Redirect: &istiov1beta1.HTTPRedirect{},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterRequestRedirect,
										RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
											Scheme:     common.PtrTo[string]("http"),
											StatusCode: common.PtrTo[int](302),
											Path: &gatewayv1.HTTPPathModifier{
												Type:            gatewayv1.FullPathHTTPPathModifier,
												ReplaceFullPath: common.PtrTo[string]("redirect-uri"),
											},
											Port: common.PtrTo[gatewayv1.PortNumber](8080),
										},
									},
								},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-1",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterRequestRedirect,
										RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
											StatusCode: common.PtrTo[int](301),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "route.Rewrite is converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Rewrite: &istiov1beta1.HTTPRewrite{
							Uri: "redirect1",
						},
					},
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								Uri: &istiov1beta1.StringMatch{
									MatchType: &istiov1beta1.StringMatch_Exact{
										Exact: "exact",
									},
								},
							},
						},
						Rewrite: &istiov1beta1.HTTPRewrite{
							Uri: "redirect2",
						},
					},
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								Uri: &istiov1beta1.StringMatch{
									MatchType: &istiov1beta1.StringMatch_Regex{
										Regex: "regex",
									},
								},
							},
						},
						Rewrite: &istiov1beta1.HTTPRewrite{
							Uri: "redirect3",
						},
					},
					{
						Match: []*istiov1beta1.HTTPMatchRequest{
							{
								Uri: &istiov1beta1.StringMatch{
									MatchType: &istiov1beta1.StringMatch_Prefix{
										Prefix: "prefix",
									},
								},
							},
						},
						Rewrite: &istiov1beta1.HTTPRewrite{
							Uri: "redirect4",
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0-prefix-match",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterURLRewrite,
										URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
											Path: &gatewayv1.HTTPPathModifier{
												Type:               gatewayv1.PrefixMatchHTTPPathModifier,
												ReplacePrefixMatch: common.PtrTo[string]("redirect1"),
											},
										},
									},
								},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-1",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo[gatewayv1.PathMatchType](gatewayv1.PathMatchExact),
											Value: common.PtrTo[string]("exact"),
										},
									},
								},
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterURLRewrite,
										URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
											Path: &gatewayv1.HTTPPathModifier{
												Type:            gatewayv1.FullPathHTTPPathModifier,
												ReplaceFullPath: common.PtrTo[string]("redirect2"),
											},
										},
									},
								},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-2",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo[gatewayv1.PathMatchType](gatewayv1.PathMatchRegularExpression),
											Value: common.PtrTo[string]("regex"),
										},
									},
								},
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterURLRewrite,
										URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
											Path: &gatewayv1.HTTPPathModifier{
												Type:            gatewayv1.FullPathHTTPPathModifier,
												ReplaceFullPath: common.PtrTo[string]("redirect3"),
											},
										},
									},
								},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-3-prefix-match",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo[gatewayv1.PathMatchType](gatewayv1.PathMatchPathPrefix),
											Value: common.PtrTo[string]("prefix"),
										},
									},
								},
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterURLRewrite,
										URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
											Path: &gatewayv1.HTTPPathModifier{
												Type:               gatewayv1.PrefixMatchHTTPPathModifier,
												ReplacePrefixMatch: common.PtrTo[string]("redirect4"),
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
		{
			name: "route.Mirror is converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Mirror: &istiov1beta1.Destination{
							Host: "reviews.prod.svc.cluster.local",
							Port: &istiov1beta1.PortSelector{
								Number: 5555,
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterRequestMirror,
										RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
											BackendRef: gatewayv1.BackendObjectReference{
												Name:      "reviews",
												Namespace: common.PtrTo[gatewayv1.Namespace]("prod"),
												Port:      common.PtrTo[gatewayv1.PortNumber](5555),
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
		{
			name: "route.Mirrors are converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Mirrors: []*istiov1beta1.HTTPMirrorPolicy{
							{
								Destination: &istiov1beta1.Destination{
									Host: "reviews.prod.svc.cluster.local",
									Port: &istiov1beta1.PortSelector{
										Number: 5555,
									},
								},
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterRequestMirror,
										RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
											BackendRef: gatewayv1.BackendObjectReference{
												Name:      "reviews",
												Namespace: common.PtrTo[gatewayv1.Namespace]("prod"),
												Port:      common.PtrTo[gatewayv1.PortNumber](5555),
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
		{
			name: "route.Timeout is converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Timeout: durationpb.New(time.Minute),
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Timeouts: &gatewayv1.HTTPRouteTimeouts{
									Request: common.PtrTo[gatewayv1.Duration]("1m0s"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "route.Headers are converted",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Headers: &istiov1beta1.Headers{
							Request: &istiov1beta1.Headers_HeaderOperations{
								Set: map[string]string{
									"h1": "v1",
								},
								Add: map[string]string{
									"h2": "v2",
								},
								Remove: []string{"h3"},
							},
							Response: &istiov1beta1.Headers_HeaderOperations{
								Set: map[string]string{
									"h4": "v4",
								},
								Add: map[string]string{
									"h5": "v5",
								},
								Remove: []string{"h6"},
							},
						},
					},
				},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
										RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
											Set: []gatewayv1.HTTPHeader{
												{
													Name:  "h1",
													Value: "v1",
												},
											},
											Add: []gatewayv1.HTTPHeader{
												{
													Name:  "h2",
													Value: "v2",
												},
											},
											Remove: []string{"h3"},
										},
									},
									{
										Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
										ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
											Set: []gatewayv1.HTTPHeader{
												{
													Name:  "h4",
													Value: "v4",
												},
											},
											Add: []gatewayv1.HTTPHeader{
												{
													Name:  "h5",
													Value: "v5",
												},
											},
											Remove: []string{"h6"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "hosts with '*'",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
					},
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Headers: &istiov1beta1.Headers{
							Request: &istiov1beta1.Headers_HeaderOperations{
								Add: map[string]string{
									"h2": "v2",
								},
							},
						},
					},
				},
				allowedHostnames: []string{"*"},
			},
			want: []*gatewayv1.HTTPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HTTPRoute",
						APIVersion: "gateway.networking.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-idx-0",
						Namespace: "ns",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{
							{
								Filters: []gatewayv1.HTTPRouteFilter{
									{
										Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
										RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
											Add: []gatewayv1.HTTPHeader{
												{
													Name:  "h2",
													Value: "v2",
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &resourcesToIRConverter{ctx: context.Background()}
			c.ctx = context.WithValue(c.ctx, virtualServiceKey, tt.args.virtualService)
			httpRoutes, errList := c.convertVsHTTPRoutes(tt.args.virtualService.ObjectMeta, tt.args.istioHTTPRoutes, tt.args.allowedHostnames, field.NewPath(""))
			if tt.wantError && len(errList) == 0 {
				t.Errorf("resourcesToIRConverter.convertVsHTTPRoutes().errList = %+v, wantError %+v", errList, tt.wantError)
			}
			if !apiequality.Semantic.DeepEqual(httpRoutes, tt.want) {
				t.Errorf("resourcesToIRConverter.convertVsHTTPRoutes().httpRoutes = %v, want %v, diff (-want +got): %s", httpRoutes, tt.want, cmp.Diff(tt.want, httpRoutes))
			}
		})
	}
}

func Test_resourcesToIRConverter_convertVsTLSRoutes(t *testing.T) {
	type args struct {
		virtualService *istioclientv1beta1.VirtualService
		istioTLSRoutes []*istiov1beta1.TLSRoute
	}
	tests := []struct {
		name string
		args args
		want []*gatewayv1alpha2.TLSRoute
	}{
		{
			name: "supported spec",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Namespace:   "ns",
						Labels:      map[string]string{"k": "v"},
						Annotations: map[string]string{"k1": "v1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "object",
							},
						},
						Finalizers: []string{"finalizer1"},
					},
				},
				istioTLSRoutes: []*istiov1beta1.TLSRoute{
					{
						Match: []*istiov1beta1.TLSMatchAttributes{
							{
								SniHosts: []string{"*.com", "test.net"},
							},
							{
								SniHosts: []string{"*.wk.org"},
							},
						},
						Route: []*istiov1beta1.RouteDestination{
							{
								Destination: &istiov1beta1.Destination{
									Host: "mongo.backup.svc.cluster.local",
									Port: &istiov1beta1.PortSelector{
										Number: 5555,
									},
								},
								Weight: 50,
							},
							{
								Destination: &istiov1beta1.Destination{
									Host: "mongo-ab.backup.svc.cluster.local",
									Port: &istiov1beta1.PortSelector{
										Number: 6555,
									},
								},
								Weight: 50,
							},
						},
					},
				},
			},
			want: []*gatewayv1alpha2.TLSRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TLSRoute",
						APIVersion: "gateway.networking.k8s.io/v1alpha2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-idx-0",
						Namespace:   "ns",
						Labels:      map[string]string{"k": "v"},
						Annotations: map[string]string{"k1": "v1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "object",
							},
						},
						Finalizers: []string{"finalizer1"},
					},
					Spec: gatewayv1alpha2.TLSRouteSpec{
						Hostnames: []gatewayv1alpha2.Hostname{
							gatewayv1alpha2.Hostname("*.com"),
							gatewayv1alpha2.Hostname("*.wk.org"),
							gatewayv1alpha2.Hostname("test.net"),
						},
						Rules: []gatewayv1alpha2.TLSRouteRule{
							{
								BackendRefs: []gatewayv1.BackendRef{
									{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name:      "mongo",
											Namespace: common.PtrTo[gatewayv1.Namespace]("backup"),
											Port:      common.PtrTo[gatewayv1.PortNumber](5555),
										},
										Weight: common.PtrTo[int32](50),
									},
									{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name:      "mongo-ab",
											Namespace: common.PtrTo[gatewayv1.Namespace]("backup"),
											Port:      common.PtrTo[gatewayv1.PortNumber](6555),
										},
										Weight: common.PtrTo[int32](50),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &resourcesToIRConverter{ctx: context.Background()}
			c.ctx = context.WithValue(c.ctx, virtualServiceKey, tt.args.virtualService)
			if got := c.convertVsTLSRoutes(tt.args.virtualService.ObjectMeta, tt.args.istioTLSRoutes, field.NewPath("")); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("resourcesToIRConverter.convertVsTLSRoutes() = %+v, want %+v, diff (-want +got): %s", got, tt.want, cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_resourcesToIRConverter_convertVsTCPRoutes(t *testing.T) {
	type args struct {
		virtualService *istioclientv1beta1.VirtualService
		istioTCPRoutes []*istiov1beta1.TCPRoute
	}
	tests := []struct {
		name string
		args args
		want []*gatewayv1alpha2.TCPRoute
	}{
		{
			name: "supported spec",
			args: args{
				virtualService: &istioclientv1beta1.VirtualService{
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Namespace:   "ns",
						Labels:      map[string]string{"k": "v"},
						Annotations: map[string]string{"k1": "v1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "object",
							},
						},
						Finalizers: []string{"finalizer1"},
					},
				},
				istioTCPRoutes: []*istiov1beta1.TCPRoute{
					{
						Route: []*istiov1beta1.RouteDestination{
							{
								Destination: &istiov1beta1.Destination{
									Host: "mongo.backup.svc.cluster.local",
									Port: &istiov1beta1.PortSelector{
										Number: 5555,
									},
								},
								Weight: 50,
							},
							{
								Destination: &istiov1beta1.Destination{
									Host: "mongo-ab.backup.svc.cluster.local",
									Port: &istiov1beta1.PortSelector{
										Number: 6555,
									},
								},
								Weight: 50,
							},
						},
					},
				},
			},
			want: []*gatewayv1alpha2.TCPRoute{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TCPRoute",
						APIVersion: "gateway.networking.k8s.io/v1alpha2",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-idx-0",
						Namespace:   "ns",
						Labels:      map[string]string{"k": "v"},
						Annotations: map[string]string{"k1": "v1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "object",
							},
						},
						Finalizers: []string{"finalizer1"},
					},
					Spec: gatewayv1alpha2.TCPRouteSpec{
						Rules: []gatewayv1alpha2.TCPRouteRule{
							{
								BackendRefs: []gatewayv1.BackendRef{
									{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name:      "mongo",
											Namespace: common.PtrTo[gatewayv1.Namespace]("backup"),
											Port:      common.PtrTo[gatewayv1.PortNumber](5555),
										},
										Weight: common.PtrTo[int32](50),
									},
									{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name:      "mongo-ab",
											Namespace: common.PtrTo[gatewayv1.Namespace]("backup"),
											Port:      common.PtrTo[gatewayv1.PortNumber](6555),
										},
										Weight: common.PtrTo[int32](50),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &resourcesToIRConverter{ctx: context.Background()}
			c.ctx = context.WithValue(c.ctx, virtualServiceKey, tt.args.virtualService)
			if got := c.convertVsTCPRoutes(tt.args.virtualService.ObjectMeta, tt.args.istioTCPRoutes, field.NewPath("")); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("resourcesToIRConverter.convertVsTCPRoutes() = %+v, want %+v, diff (-want +got): %s", got, tt.want, cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestNameMatches(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		out  bool
	}{
		{"empty", "", "", true},
		{"first empty", "", "foo.com", false},
		{"second empty", "foo.com", "", false},

		{
			"non-wildcard domain",
			"foo.com", "foo.com", true,
		},
		{
			"non-wildcard domain",
			"bar.com", "foo.com", false,
		},
		{
			"non-wildcard domain - order doesn't matter",
			"foo.com", "bar.com", false,
		},

		{
			"domain does not match subdomain",
			"bar.foo.com", "foo.com", false,
		},
		{
			"domain does not match subdomain - order doesn't matter",
			"foo.com", "bar.foo.com", false,
		},

		{
			"wildcard matches subdomains",
			"*.com", "foo.com", true,
		},
		{
			"wildcard matches subdomains",
			"*.com", "bar.com", true,
		},
		{
			"wildcard matches subdomains",
			"*.foo.com", "bar.foo.com", true,
		},

		{"wildcard matches anything", "*", "foo.com", true},
		{"wildcard matches anything", "*", "*.com", true},
		{"wildcard matches anything", "*", "com", true},
		{"wildcard matches anything", "*", "*", true},
		{"wildcard matches anything", "*", "", true},

		{"wildcarded domain matches wildcarded subdomain", "*.com", "*.foo.com", true},
		{"wildcarded sub-domain does not match domain", "foo.com", "*.foo.com", false},
		{"wildcarded sub-domain does not match domain - order doesn't matter", "*.foo.com", "foo.com", false},

		{"long wildcard does not match short host", "*.foo.bar.baz", "baz", false},
		{"long wildcard does not match short host - order doesn't matter", "baz", "*.foo.bar.baz", false},
		{"long wildcard matches short wildcard", "*.foo.bar.baz", "*.baz", true},
		{"long name matches short wildcard", "foo.bar.baz", "*.baz", true},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			if tt.out != matches(tt.a, tt.b) {
				t.Errorf("matches(%q, %q) = %t wanted %t", tt.a, tt.b, !tt.out, tt.out)
			}

			if tt.out != matches(tt.b, tt.a) {
				t.Errorf("symmetrical: matches(%q, %q) = %t wanted %t", tt.b, tt.a, !tt.out, tt.out)
			}
		})
	}
}

func Test_resourcesToIRConverter_generateReferenceGrants(t *testing.T) {
	type args struct {
		params generateReferenceGrantsParams
	}
	tests := []struct {
		name string
		args args
		want *gatewayv1beta1.ReferenceGrant
	}{
		{
			name: "generate reference grant for HTTPRoute,TLSRoute,TCPRoute",
			args: args{
				params: generateReferenceGrantsParams{
					gateway: types.NamespacedName{
						Namespace: "test",
						Name:      "gwname",
					},
					fromNamespace: "ns1",
					forHTTPRoute:  true,
					forTLSRoute:   true,
					forTCPRoute:   true,
				},
			},
			want: &gatewayv1beta1.ReferenceGrant{
				TypeMeta: metav1.TypeMeta{
					APIVersion: common.ReferenceGrantGVK.GroupVersion().String(),
					Kind:       common.ReferenceGrantGVK.Kind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "generated-reference-grant-from-ns1-to-test",
				},
				Spec: gatewayv1beta1.ReferenceGrantSpec{
					From: []gatewayv1beta1.ReferenceGrantFrom{
						{
							Group:     gatewayv1.Group(common.HTTPRouteGVK.Group),
							Kind:      gatewayv1.Kind(common.HTTPRouteGVK.Kind),
							Namespace: gatewayv1.Namespace("ns1"),
						},
						{
							Group:     gatewayv1.Group(common.TLSRouteGVK.Group),
							Kind:      gatewayv1.Kind(common.TLSRouteGVK.Kind),
							Namespace: gatewayv1.Namespace("ns1"),
						},
						{
							Group:     gatewayv1.Group(common.TCPRouteGVK.Group),
							Kind:      gatewayv1.Kind(common.TCPRouteGVK.Kind),
							Namespace: gatewayv1.Namespace("ns1"),
						},
					},
					To: []gatewayv1beta1.ReferenceGrantTo{
						{
							Group: gatewayv1.Group(common.GatewayGVK.Group),
							Kind:  gatewayv1.Kind(common.GatewayGVK.Kind),
							Name:  common.PtrTo[gatewayv1.ObjectName]("gwname"),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &resourcesToIRConverter{}
			if got := c.generateReferenceGrant(tt.args.params); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("resourcesToIRConverter.generateReferenceGrant() = %+v, want %+v, diff (-want +got): %s", got, tt.want, cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_resourcesToIRConverter_isGatewayAllowedForVirtualService(t *testing.T) {
	type fields struct {
		gwAllowedHosts map[types.NamespacedName]map[string]sets.Set[string]
	}
	type args struct {
		gateway types.NamespacedName
		vs      *istioclientv1beta1.VirtualService
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "ignore mesh gateway",
			args: args{
				gateway: types.NamespacedName{
					Name: "mesh",
				},
				vs: &istioclientv1beta1.VirtualService{Spec: istiov1beta1.VirtualService{}},
			},
			want: false,
		},
		{
			name: "gateway is not allowed -- different namespace",
			args: args{
				gateway: types.NamespacedName{
					Namespace: "test",
				},
				vs: &istioclientv1beta1.VirtualService{Spec: istiov1beta1.VirtualService{
					ExportTo: []string{"prod", "."},
				}},
			},
			want: false,
		},
		{
			name: "unknown gateway is not allowed",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						"prodv1": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				gateway: types.NamespacedName{
					Namespace: "prod",
					Name:      "gateway1",
				},
				vs: &istioclientv1beta1.VirtualService{Spec: istiov1beta1.VirtualService{
					ExportTo: []string{"*"},
				}},
			},
			want: false,
		},
		{
			name: "gateway is allowed -- namespace match and match by host",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						"prod": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				gateway: types.NamespacedName{
					Namespace: "prod",
					Name:      "gateway",
				},
				vs: &istioclientv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "prod",
					},
					Spec: istiov1beta1.VirtualService{
						ExportTo: []string{"*"},
						Hosts:    []string{"prod.com"},
					}},
			},
			want: true,
		},
		{
			name: "gateway is allowed -- . namespace for the host and namespaces are equal",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						".": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				gateway: types.NamespacedName{
					Namespace: "prod",
					Name:      "gateway",
				},
				vs: &istioclientv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "prod",
					},
					Spec: istiov1beta1.VirtualService{
						ExportTo: []string{"*"},
						Hosts:    []string{"prod.com"},
					}},
			},
			want: true,
		},
		{
			name: "gateway is allowed -- * namespace for the host and namespaces are equal",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						"*": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				gateway: types.NamespacedName{
					Namespace: "prod",
					Name:      "gateway",
				},
				vs: &istioclientv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
					Spec: istiov1beta1.VirtualService{
						ExportTo: []string{"*"},
						Hosts:    []string{"prod.com"},
					}},
			},
			want: true,
		},
		{
			name: "gateway is not allowed -- . namespace is allowed but the virtualService has another ns",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						".": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				gateway: types.NamespacedName{
					Namespace: "prod",
					Name:      "gateway",
				},
				vs: &istioclientv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
					Spec: istiov1beta1.VirtualService{
						ExportTo: []string{"*"},
						Hosts:    []string{"prod.com"},
					}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &resourcesToIRConverter{
				gwAllowedHosts: tt.fields.gwAllowedHosts,
			}
			if got := c.isVirtualServiceAllowedForGateway(tt.args.gateway, tt.args.vs, field.NewPath("")); got != tt.want {
				t.Errorf("resourcesToIRConverter.isVirtualServiceAllowedForGateway() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resourcesToIRConverter_generateReferences(t *testing.T) {
	type fields struct {
		gwAllowedHosts map[types.NamespacedName]map[string]sets.Set[string]
	}
	type args struct {
		vs *istioclientv1beta1.VirtualService
	}
	tests := []struct {
		name                 string
		fields               fields
		args                 args
		wantParentReferences []gatewayv1.ParentReference
		wantReferenceGrants  []*gatewayv1beta1.ReferenceGrant
	}{
		{
			name: "nothing is generated if Gateway is not listed in the VirtualService",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						"*": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				vs: &istioclientv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					Spec: istiov1beta1.VirtualService{
						ExportTo: []string{"*"},
						Hosts:    []string{"prod.com"},
						Gateways: []string{"gateway", "prodv1/gateway"},
					}},
			},
		},
		{
			name: "generate referenceGrant for the allowed VirtualService",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						"*": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				vs: &istioclientv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					Spec: istiov1beta1.VirtualService{
						ExportTo: []string{"*"},
						Hosts:    []string{"prod.com"},
						Gateways: []string{"prod/gateway"},
					}},
			},
			wantParentReferences: []gatewayv1.ParentReference{
				{
					Group:     common.PtrTo[gatewayv1.Group]("gateway.networking.k8s.io"),
					Kind:      common.PtrTo[gatewayv1.Kind]("Gateway"),
					Namespace: common.PtrTo[gatewayv1.Namespace]("prod"),
					Name:      "gateway",
				},
			},
			wantReferenceGrants: []*gatewayv1beta1.ReferenceGrant{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "ReferenceGrant", APIVersion: "gateway.networking.k8s.io/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "generated-reference-grant-from-test-to-prod", Namespace: "prod"},
					Spec: gatewayv1beta1.ReferenceGrantSpec{
						To: []gatewayv1beta1.ReferenceGrantTo{
							{
								Group: "gateway.networking.k8s.io", Kind: "Gateway", Name: common.PtrTo[gatewayv1.ObjectName]("gateway"),
							},
						},
					},
				},
			},
		},
		{
			name: "generate only parentRef for the allowed VirtualService same ns as the Gateway",
			fields: fields{
				gwAllowedHosts: map[types.NamespacedName]map[string]sets.Set[string]{
					{
						Namespace: "prod",
						Name:      "gateway",
					}: {
						"*": sets.New[string]("prod.com", "*.v1.prod.com"),
					},
				},
			},
			args: args{
				vs: &istioclientv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "prod",
					},
					TypeMeta: metav1.TypeMeta{
						Kind: "VirtualService",
					},
					Spec: istiov1beta1.VirtualService{
						ExportTo: []string{"*"},
						Hosts:    []string{"prod.com"},
						Gateways: []string{"prod/gateway"},
					}},
			},
			wantParentReferences: []gatewayv1.ParentReference{
				{
					Group: common.PtrTo[gatewayv1.Group]("gateway.networking.k8s.io"),
					Kind:  common.PtrTo[gatewayv1.Kind]("Gateway"),
					Name:  "gateway",
				},
			},
			wantReferenceGrants: []*gatewayv1beta1.ReferenceGrant{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &resourcesToIRConverter{
				gwAllowedHosts: tt.fields.gwAllowedHosts,
			}
			gotParentReferences, gotReferenceGrants := c.generateReferences(tt.args.vs, field.NewPath(""))
			if !apiequality.Semantic.DeepEqual(gotParentReferences, tt.wantParentReferences) {
				t.Errorf("resourcesToIRConverter.generateReferences() gotParentReferences = %v, want %v, diff (-want +got): %s", gotParentReferences, tt.wantParentReferences, cmp.Diff(tt.wantParentReferences, gotParentReferences))
			}
			if !apiequality.Semantic.DeepEqual(gotReferenceGrants, tt.wantReferenceGrants) {
				t.Errorf("resourcesToIRConverter.generateReferences() gotReferenceGrants = %v, want %v, diff (-want +got): %s", gotReferenceGrants, tt.wantReferenceGrants, cmp.Diff(tt.wantReferenceGrants, gotReferenceGrants))
			}
		})
	}
}

func Test_convertHostnames(t *testing.T) {
	cases := []struct {
		name           string
		virtualService *istioclientv1beta1.VirtualService
		hostnames      []string
		expected       []gatewayv1alpha2.Hostname
	}{
		{
			name: "default",
			virtualService: &istioclientv1beta1.VirtualService{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualService",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
				},
			},
			hostnames: []string{"*.com", "test.net", "*.example.com"},
			expected:  []gatewayv1alpha2.Hostname{"*.com", "test.net", "*.example.com"},
		},
		{
			name: "* is not allowed",
			virtualService: &istioclientv1beta1.VirtualService{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualService",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
				},
			},
			hostnames: []string{"*"},
			expected:  []gatewayv1alpha2.Hostname{},
		},
		{
			name: "IP is not allowed",
			virtualService: &istioclientv1beta1.VirtualService{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualService",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
				},
			},
			hostnames: []string{"192.0.2.1", "2001:db8::68", "::ffff:192.0.2.1"},
			expected:  []gatewayv1alpha2.Hostname{},
		},
		{
			name: "The wildcard label must appear by itself as the first character",
			virtualService: &istioclientv1beta1.VirtualService{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualService",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
				},
			},
			hostnames: []string{"example*.com"},

			expected: []gatewayv1alpha2.Hostname{},
		},
		{
			name: "mix",
			virtualService: &istioclientv1beta1.VirtualService{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualService",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
				},
			},
			hostnames: []string{"192.0.2.1", "2001:db8::68", "::ffff:192.0.2.1", "*", "*.com", "test.net", "*.example.com"},
			expected:  []gatewayv1alpha2.Hostname{"*.com", "test.net", "*.example.com"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), virtualServiceKey, tc.virtualService)
			actual := convertHostnames(ctx, tc.hostnames, field.NewPath(""))
			if !apiequality.Semantic.DeepEqual(actual, tc.expected) {
				t.Errorf("convertHostnames() = %v, want %v", actual, tc.expected)
			}
		})
	}
}
