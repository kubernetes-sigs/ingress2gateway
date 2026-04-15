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

func Test_routerTLSFeature(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix

	testCases := []struct {
		name             string
		ingress          networkingv1.Ingress
		initialGateway   gatewayv1.Gateway
		expectedGateway  gatewayv1.Gateway
		expectedErrCount int
	}{
		{
			name: "router.tls=true without spec.tls adds HTTPS listener with placeholder cert",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterTLSAnnotation: "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules: []networkingv1.IngressRule{{
						Host: "foo.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path: "/", PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-app",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			initialGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
					},
				},
			},
			expectedGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
						{
							Name:     "foo-com-https",
							Hostname: hostnamePtr("foo.com"),
							Port:     443,
							Protocol: gatewayv1.HTTPSProtocolType,
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
			},
		},
		{
			name: "router.tls=true with spec.tls already set — no change",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterTLSAnnotation: "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					TLS: []networkingv1.IngressTLS{{
						Hosts:      []string{"foo.com"},
						SecretName: "existing-secret",
					}},
					Rules: []networkingv1.IngressRule{{
						Host: "foo.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path: "/", PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-app",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			initialGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
						{Name: "foo-com-https", Hostname: hostnamePtr("foo.com"), Port: 443, Protocol: gatewayv1.HTTPSProtocolType,
							TLS: &gatewayv1.ListenerTLSConfig{
								CertificateRefs: []gatewayv1.SecretObjectReference{{Name: "existing-secret"}},
							},
						},
					},
				},
			},
			expectedGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
						{Name: "foo-com-https", Hostname: hostnamePtr("foo.com"), Port: 443, Protocol: gatewayv1.HTTPSProtocolType,
							TLS: &gatewayv1.ListenerTLSConfig{
								CertificateRefs: []gatewayv1.SecretObjectReference{{Name: "existing-secret"}},
							},
						},
					},
				},
			},
		},
		{
			name: "router.tls=false — no HTTPS listener added",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterTLSAnnotation: "false",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules: []networkingv1.IngressRule{{
						Host: "foo.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path: "/", PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-app",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			initialGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
					},
				},
			},
			expectedGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
					},
				},
			},
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
					Rules: []networkingv1.IngressRule{{
						Host: "foo.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path: "/", PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-app",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			initialGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
					},
				},
			},
			expectedGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
					},
				},
			},
		},
		{
			name: "router.tls=true called twice — no duplicate HTTPS listener",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						RouterTLSAnnotation: "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules: []networkingv1.IngressRule{{
						Host: "foo.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path: "/", PathType: &iPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-app",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			},
			initialGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
						// Simulate a listener already added (e.g. by a prior run)
						{
							Name:     "foo-com-https",
							Hostname: hostnamePtr("foo.com"),
							Port:     443,
							Protocol: gatewayv1.HTTPSProtocolType,
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
			},
			expectedGateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: "default"},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: "traefik",
					Listeners: []gatewayv1.Listener{
						{Name: "foo-com-http", Hostname: hostnamePtr("foo.com"), Port: 80, Protocol: gatewayv1.HTTPProtocolType},
						{
							Name:     "foo-com-https",
							Hostname: hostnamePtr("foo.com"),
							Port:     443,
							Protocol: gatewayv1.HTTPSProtocolType,
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
					gatewayKey: {Gateway: tc.initialGateway},
				},
				HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
					routeKey: {HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{Name: routeKey.Name, Namespace: routeKey.Namespace},
					}},
				},
			}

			errs := routerTLSFeature(notifications.NoopNotify, ingresses, nil, ir)

			if len(errs) != tc.expectedErrCount {
				t.Errorf("expected %d errors, got %d: %v", tc.expectedErrCount, len(errs), errs)
			}

			actualGW := ir.Gateways[gatewayKey].Gateway
			if diff := cmp.Diff(tc.expectedGateway, actualGW); diff != "" {
				t.Errorf("unexpected Gateway (-want +got):\n%s", diff)
			}
		})
	}
}

// hostnamePtr is a test helper that converts a string to a *gatewayv1.Hostname.
func hostnamePtr(h string) *gatewayv1.Hostname {
	v := gatewayv1.Hostname(h)
	return &v
}
