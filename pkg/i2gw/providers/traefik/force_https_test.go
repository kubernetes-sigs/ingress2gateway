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

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_forceHTTPSFeature(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix

	bothListeners := func(host string) []gatewayv1.Listener {
		return []gatewayv1.Listener{
			{Name: gatewayv1.SectionName(httpListenerName(host)), Hostname: hostnamePtr(host), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
			{Name: gatewayv1.SectionName(httpsListenerName(host)), Hostname: hostnamePtr(host), Port: 443, Protocol: gatewayv1.HTTPSProtocolType,
				TLS: &gatewayv1.ListenerTLSConfig{
					Mode: ptr.To(gatewayv1.TLSModeTerminate),
					CertificateRefs: []gatewayv1.SecretObjectReference{{
						Group: ptr.To(gatewayv1.Group("")),
						Kind:  ptr.To(gatewayv1.Kind("Secret")),
						Name:  "foo-com-tls",
					}},
				},
			},
		}
	}

	makeRedirectRoute := func(host, namespace, routeName string) gatewayv1.HTTPRoute {
		sectionName := gatewayv1.SectionName(httpListenerName(host))
		gwNamespace := gatewayv1.Namespace(namespace)
		route := gatewayv1.HTTPRoute{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "gateway.networking.k8s.io/v1",
				Kind:       "HTTPRoute",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      routeName + "-http",
				Namespace: namespace,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(host)},
				Rules: []gatewayv1.HTTPRouteRule{{
					Filters: []gatewayv1.HTTPRouteFilter{{
						Type: gatewayv1.HTTPRouteFilterRequestRedirect,
						RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
							Scheme:     ptr.To("https"),
							StatusCode: ptr.To(301),
						},
					}},
				}},
			},
		}
		route.Spec.ParentRefs = []gatewayv1.ParentReference{{
			Name:        "traefik",
			Namespace:   &gwNamespace,
			SectionName: &sectionName,
		}}
		return route
	}

	testCases := []struct {
		name                         string
		ingress                      networkingv1.Ingress
		listeners                    []gatewayv1.Listener
		expectedRedirectRoute        *gatewayv1.HTTPRoute
		expectedMainRouteSectionName *gatewayv1.SectionName // nil means no sectionName expected
		expectedErrCount             int
	}{
		{
			name: "websecure with HTTP+HTTPS listeners generates redirect route",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: "websecure",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(&iPrefix, "my-app")}},
				},
			},
			listeners: bothListeners("foo.com"),
			expectedRedirectRoute: func() *gatewayv1.HTTPRoute {
				r := makeRedirectRoute("foo.com", "default", "my-app-foo-com")
				return &r
			}(),
			expectedMainRouteSectionName: func() *gatewayv1.SectionName {
				s := gatewayv1.SectionName(httpsListenerName("foo.com"))
				return &s
			}(),
		},
		{
			// HTTP listener was removed by routerEntrypointsFeature (fallback) -- no redirect route.
			name: "websecure without HTTP listener generates no redirect route",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: "websecure",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(&iPrefix, "my-app")}},
				},
			},
			listeners: []gatewayv1.Listener{
				{Name: gatewayv1.SectionName(httpsListenerName("foo.com")), Hostname: hostnamePtr("foo.com"), Port: 443, Protocol: gatewayv1.HTTPSProtocolType},
			},
			expectedRedirectRoute: nil,
		},
		{
			name: "no annotation -- no redirect route",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(&iPrefix, "my-app")}},
				},
			},
			listeners:             bothListeners("foo.com"),
			expectedRedirectRoute: nil,
		},
		{
			name: "entrypoints=web -- no redirect route",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: "web",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(&iPrefix, "my-app")}},
				},
			},
			listeners:             bothListeners("foo.com"),
			expectedRedirectRoute: nil,
		},
		{
			name: "entrypoints=web,websecure -- no redirect route (HTTP explicitly included)",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: "web,websecure",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(&iPrefix, "my-app")}},
				},
			},
			listeners:             bothListeners("foo.com"),
			expectedRedirectRoute: nil,
		},
		{
			name: "entrypoints=WEBSECURE (uppercase) generates redirect route",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: "WEBSECURE",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(&iPrefix, "my-app")}},
				},
			},
			listeners: bothListeners("foo.com"),
			expectedRedirectRoute: func() *gatewayv1.HTTPRoute {
				r := makeRedirectRoute("foo.com", "default", "my-app-foo-com")
				return &r
			}(),
			expectedMainRouteSectionName: func() *gatewayv1.SectionName {
				s := gatewayv1.SectionName(httpsListenerName("foo.com"))
				return &s
			}(),
		},
		{
			name: "entrypoints=websecure with spaces generates redirect route",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: " websecure ",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue(&iPrefix, "my-app")}},
				},
			},
			listeners: bothListeners("foo.com"),
			expectedRedirectRoute: func() *gatewayv1.HTTPRoute {
				r := makeRedirectRoute("foo.com", "default", "my-app-foo-com")
				return &r
			}(),
			expectedMainRouteSectionName: func() *gatewayv1.SectionName {
				s := gatewayv1.SectionName(httpsListenerName("foo.com"))
				return &s
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingresses := []networkingv1.Ingress{tc.ingress}
			gatewayKey := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: "traefik"}
			routeKey := types.NamespacedName{
				Namespace: tc.ingress.Namespace,
				Name:      common.RouteName(tc.ingress.Name, tc.ingress.Spec.Rules[0].Host),
			}

			ir := &providerir.ProviderIR{
				Gateways: map[types.NamespacedName]providerir.GatewayContext{
					gatewayKey: {Gateway: gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: tc.ingress.Namespace},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "traefik",
							Listeners:        tc.listeners,
						},
					}},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					routeKey: {HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{Name: routeKey.Name, Namespace: routeKey.Namespace},
					}},
				},
			}

			// Set a parentRef on the main HTTPRoute so forceHTTPSFeature can pin it.
			mainCtx := ir.HTTPRoutes[routeKey]
			mainCtx.HTTPRoute.Spec.ParentRefs = []gatewayv1.ParentReference{{
				Name: gatewayv1.ObjectName("traefik"),
			}}
			ir.HTTPRoutes[routeKey] = mainCtx

			errs := forceHTTPSFeature(notifications.NoopNotify, ingresses, nil, ir)

			if len(errs) != tc.expectedErrCount {
				t.Errorf("expected %d errors, got %d: %v", tc.expectedErrCount, len(errs), errs)
			}

			redirectRouteKey := types.NamespacedName{Namespace: tc.ingress.Namespace, Name: routeKey.Name + "-http"}
			if tc.expectedRedirectRoute != nil {
				redirectCtx, ok := ir.HTTPRoutes[redirectRouteKey]
				if !ok {
					t.Errorf("expected redirect HTTPRoute %v to be present, but it was not", redirectRouteKey)
				} else if diff := cmp.Diff(*tc.expectedRedirectRoute, redirectCtx.HTTPRoute); diff != "" {
					t.Errorf("unexpected redirect HTTPRoute (-want +got):\n%s", diff)
				}
			} else {
				if _, ok := ir.HTTPRoutes[redirectRouteKey]; ok {
					t.Errorf("expected no redirect HTTPRoute %v, but one was found", redirectRouteKey)
				}
			}

			// Assert the main HTTPRoute is pinned to the HTTPS listener when expected.
			mainRoute := ir.HTTPRoutes[routeKey]
			if tc.expectedMainRouteSectionName != nil {
				if len(mainRoute.Spec.ParentRefs) == 0 || mainRoute.Spec.ParentRefs[0].SectionName == nil {
					t.Errorf("expected main HTTPRoute parentRef to have sectionName %q, but it was nil", *tc.expectedMainRouteSectionName)
				} else if *mainRoute.Spec.ParentRefs[0].SectionName != *tc.expectedMainRouteSectionName {
					t.Errorf("expected main HTTPRoute sectionName %q, got %q", *tc.expectedMainRouteSectionName, *mainRoute.Spec.ParentRefs[0].SectionName)
				}
			} else {
				if len(mainRoute.Spec.ParentRefs) > 0 && mainRoute.Spec.ParentRefs[0].SectionName != nil {
					t.Errorf("expected main HTTPRoute parentRef to have no sectionName, but got %q", *mainRoute.Spec.ParentRefs[0].SectionName)
				}
			}
		})
	}
}
