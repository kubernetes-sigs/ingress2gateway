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

func Test_converter_convertHTTPRoutes(t *testing.T) {
	type args struct {
		virtualService   metav1.ObjectMeta
		istioHTTPRoutes  []*istiov1beta1.HTTPRoute
		allowedHostnames []string
	}
	tests := []struct {
		name string
		args args
		want []*gatewayv1.HTTPRoute
	}{
		{
			name: "objectMeta field is converted and hosts are set",
			args: args{
				virtualService: metav1.ObjectMeta{
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
				},
				istioHTTPRoutes: []*istiov1beta1.HTTPRoute{
					{
						Rewrite: &istiov1beta1.HTTPRewrite{
							Uri: "redirect-uri",
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
										Type: gatewayv1.HTTPRouteFilterURLRewrite,
										URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
											Path: &gatewayv1.HTTPPathModifier{
												Type:               gatewayv1.PrefixMatchHTTPPathModifier,
												ReplacePrefixMatch: common.PtrTo[string]("redirect-uri"),
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
				virtualService: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &converter{}
			if got := c.convertHTTPRoutes(tt.args.virtualService, tt.args.istioHTTPRoutes, tt.args.allowedHostnames, field.NewPath("")); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("converter.convertHTTPRoutes() = %v, want %v, diff (-want +got): %s", got, tt.want, cmp.Diff(tt.want, got))
			}
		})
	}
}
