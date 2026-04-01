/*
Copyright 2024 The Kubernetes Authors.

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

func Test_routerEntrypointsFeature(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix

	// bothListeners is the Gateway state that common.ToIR() produces when
	// a host has TLS configured — HTTP on 80 and HTTPS on 443.
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

	testCases := []struct {
		name              string
		ingress           networkingv1.Ingress
		initialListeners  []gatewayv1.Listener
		expectedListeners []gatewayv1.Listener
		expectedErrCount  int
	}{
		{
			name: "entrypoints=websecure removes HTTP listener",
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
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners: bothListeners("foo.com"),
			expectedListeners: []gatewayv1.Listener{
				{Name: gatewayv1.SectionName(httpsListenerName("foo.com")), Hostname: hostnamePtr("foo.com"), Port: 443, Protocol: gatewayv1.HTTPSProtocolType,
					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: ptr.To(gatewayv1.TLSModeTerminate),
						CertificateRefs: []gatewayv1.SecretObjectReference{{
							Group: ptr.To(gatewayv1.Group("")),
							Kind:  ptr.To(gatewayv1.Kind("Secret")),
							Name:  "foo-com-tls",
						}},
					},
				},
			},
		},
		{
			name: "entrypoints=web removes HTTPS listener",
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
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners: bothListeners("foo.com"),
			expectedListeners: []gatewayv1.Listener{
				{Name: gatewayv1.SectionName(httpListenerName("foo.com")), Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
			},
		},
		{
			name: "entrypoints=web,websecure keeps both listeners",
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
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners:  bothListeners("foo.com"),
			expectedListeners: bothListeners("foo.com"),
		},
		{
			name: "entrypoints=websecure,web (reversed order) keeps both listeners",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: "websecure,web",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners:  bothListeners("foo.com"),
			expectedListeners: bothListeners("foo.com"),
		},
		{
			name: "non-standard entrypoint emits warning and keeps listeners unchanged",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterEntrypointsAnnotation: "internal",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners:  bothListeners("foo.com"),
			expectedListeners: bothListeners("foo.com"),
		},
		{
			name: "no annotation — no change",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners:  bothListeners("foo.com"),
			expectedListeners: bothListeners("foo.com"),
		},
		{
			name: "entrypoints=WEBSECURE (uppercase) removes HTTP listener",
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
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners: bothListeners("foo.com"),
			expectedListeners: []gatewayv1.Listener{
				{Name: gatewayv1.SectionName(httpsListenerName("foo.com")), Hostname: hostnamePtr("foo.com"), Port: 443, Protocol: gatewayv1.HTTPSProtocolType,
					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: ptr.To(gatewayv1.TLSModeTerminate),
						CertificateRefs: []gatewayv1.SecretObjectReference{{
							Group: ptr.To(gatewayv1.Group("")),
							Kind:  ptr.To(gatewayv1.Kind("Secret")),
							Name:  "foo-com-tls",
						}},
					},
				},
			},
		},
		{
			name: "entrypoints=websecure with spaces trimmed removes HTTP listener",
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
					Rules:            []networkingv1.IngressRule{{Host: "foo.com", IngressRuleValue: ingressRuleValue("/", &iPrefix, "my-app", 80)}},
				},
			},
			initialListeners: bothListeners("foo.com"),
			expectedListeners: []gatewayv1.Listener{
				{Name: gatewayv1.SectionName(httpsListenerName("foo.com")), Hostname: hostnamePtr("foo.com"), Port: 443, Protocol: gatewayv1.HTTPSProtocolType,
					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: ptr.To(gatewayv1.TLSModeTerminate),
						CertificateRefs: []gatewayv1.SecretObjectReference{{
							Group: ptr.To(gatewayv1.Group("")),
							Kind:  ptr.To(gatewayv1.Kind("Secret")),
							Name:  "foo-com-tls",
						}},
					},
				},
			},
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
							Listeners:        tc.initialListeners,
						},
					}},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					routeKey: {HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{Name: routeKey.Name, Namespace: routeKey.Namespace},
					}},
				},
			}

			errs := routerEntrypointsFeature(notifications.NoopNotify, ingresses, nil, ir)

			if len(errs) != tc.expectedErrCount {
				t.Errorf("expected %d errors, got %d: %v", tc.expectedErrCount, len(errs), errs)
			}

			actualListeners := ir.Gateways[gatewayKey].Gateway.Spec.Listeners
			if diff := cmp.Diff(tc.expectedListeners, actualListeners); diff != "" {
				t.Errorf("unexpected Gateway listeners (-want +got):\n%s", diff)
			}
		})
	}
}

// ingressRuleValue is a test helper that builds an IngressRuleValue with a single path.
func ingressRuleValue(path string, pathType *networkingv1.PathType, svcName string, port int32) networkingv1.IngressRuleValue {
	return networkingv1.IngressRuleValue{
		HTTP: &networkingv1.HTTPIngressRuleValue{
			Paths: []networkingv1.HTTPIngressPath{{
				Path:     path,
				PathType: pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: svcName,
						Port: networkingv1.ServiceBackendPort{Number: port},
					},
				},
			}},
		},
	}
}
