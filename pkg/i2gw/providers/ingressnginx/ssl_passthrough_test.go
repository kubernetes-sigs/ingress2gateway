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

package ingressnginx

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var testNotify = notifications.NoopNotify

func TestSSLPassthroughFeature(t *testing.T) {
	t.Run("basic ssl-passthrough creates TLSRoute and removes HTTPRoute", func(t *testing.T) {
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: "default",
				Annotations: map[string]string{
					SSLPassthroughAnnotation: "true",
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("nginx"),
				TLS: []networkingv1.IngressTLS{{
					Hosts: []string{"example.com"},
				}},
				Rules: []networkingv1.IngressRule{{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "my-service",
										Port: networkingv1.ServiceBackendPort{Number: 443},
									},
								},
							}},
						},
					},
				}},
			},
		}

		port443 := gatewayv1.PortNumber(443)
		httpRouteKey := types.NamespacedName{Namespace: "default", Name: "test-ingress-example-com"}
		gwKey := types.NamespacedName{Namespace: "default", Name: "nginx"}

		ir := &providerir.ProviderIR{
			Gateways: map[types.NamespacedName]providerir.GatewayContext{
				gwKey: {
					Gateway: gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "nginx",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "nginx",
							Listeners: []gatewayv1.Listener{
								{
									Name:     "example-com-http",
									Hostname: common.PtrTo(gatewayv1.Hostname("example.com")),
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
								},
								{
									Name:     "example-com-https",
									Hostname: common.PtrTo(gatewayv1.Hostname("example.com")),
									Port:     443,
									Protocol: gatewayv1.HTTPSProtocolType,
									TLS:      &gatewayv1.ListenerTLSConfig{},
								},
							},
						},
					},
				},
			},
			HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
				httpRouteKey: {
					HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ingress-example-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: "nginx",
								}},
							},
							Hostnames: []gatewayv1.Hostname{"example.com"},
							Rules: []gatewayv1.HTTPRouteRule{{
								Matches: []gatewayv1.HTTPRouteMatch{{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
										Value: ptr.To("/"),
									},
								}},
								BackendRefs: []gatewayv1.HTTPBackendRef{{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "my-service",
											Port: &port443,
										},
									},
								}},
							}},
						},
					},
					RuleBackendSources: [][]providerir.BackendSource{
						{{Ingress: &ingress}},
					},
				},
			},
			TLSRoutes: make(map[types.NamespacedName]gatewayv1.TLSRoute),
			Services:  make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
		}

		errs := sslPassthroughFeature(testNotify, []networkingv1.Ingress{ingress}, nil, ir)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}

		// HTTPRoute should be removed.
		if _, exists := ir.HTTPRoutes[httpRouteKey]; exists {
			t.Fatal("expected HTTPRoute to be removed after ssl-passthrough conversion")
		}

		// TLSRoute should be created.
		tlsRouteKey := types.NamespacedName{Namespace: "default", Name: "test-ingress-example-com-tls-passthrough"}
		tlsRoute, exists := ir.TLSRoutes[tlsRouteKey]
		if !exists {
			t.Fatalf("expected TLSRoute %v to be created, got keys: %v", tlsRouteKey, ir.TLSRoutes)
		}

		// Verify TLSRoute spec.
		if len(tlsRoute.Spec.Hostnames) != 1 || tlsRoute.Spec.Hostnames[0] != "example.com" {
			t.Errorf("expected hostname example.com, got %v", tlsRoute.Spec.Hostnames)
		}
		if len(tlsRoute.Spec.Rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(tlsRoute.Spec.Rules))
		}
		if len(tlsRoute.Spec.Rules[0].BackendRefs) != 1 {
			t.Fatalf("expected 1 backend ref, got %d", len(tlsRoute.Spec.Rules[0].BackendRefs))
		}
		if tlsRoute.Spec.Rules[0].BackendRefs[0].Name != "my-service" {
			t.Errorf("expected backend name my-service, got %s", tlsRoute.Spec.Rules[0].BackendRefs[0].Name)
		}
		if *tlsRoute.Spec.Rules[0].BackendRefs[0].Port != 443 {
			t.Errorf("expected backend port 443, got %d", *tlsRoute.Spec.Rules[0].BackendRefs[0].Port)
		}

		// Verify parent ref points to passthrough section.
		if len(tlsRoute.Spec.ParentRefs) != 1 {
			t.Fatalf("expected 1 parent ref, got %d", len(tlsRoute.Spec.ParentRefs))
		}
		expectedSection := gatewayv1.SectionName("example-com-tls-passthrough")
		if tlsRoute.Spec.ParentRefs[0].SectionName == nil || *tlsRoute.Spec.ParentRefs[0].SectionName != expectedSection {
			t.Errorf("expected section name %q, got %v", expectedSection, tlsRoute.Spec.ParentRefs[0].SectionName)
		}

		// Verify Gateway has a TLS Passthrough listener.
		gw := ir.Gateways[gwKey]
		var foundPassthroughListener bool
		for _, l := range gw.Gateway.Spec.Listeners {
			if l.Name == "example-com-tls-passthrough" {
				foundPassthroughListener = true
				if l.Protocol != gatewayv1.TLSProtocolType {
					t.Errorf("expected TLS protocol, got %s", l.Protocol)
				}
				if l.Port != 443 {
					t.Errorf("expected port 443, got %d", l.Port)
				}
				if l.TLS == nil || l.TLS.Mode == nil || *l.TLS.Mode != gatewayv1.TLSModePassthrough {
					t.Errorf("expected TLS mode Passthrough, got %v", l.TLS)
				}
				if l.Hostname == nil || *l.Hostname != "example.com" {
					t.Errorf("expected hostname example.com on listener, got %v", l.Hostname)
				}
				break
			}
		}
		if !foundPassthroughListener {
			t.Errorf("expected a TLS Passthrough listener on the Gateway, found listeners: %+v", gw.Gateway.Spec.Listeners)
		}
	})

	t.Run("ssl-passthrough false is ignored", func(t *testing.T) {
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: "default",
				Annotations: map[string]string{
					SSLPassthroughAnnotation: "false",
				},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "my-service",
										Port: networkingv1.ServiceBackendPort{Number: 443},
									},
								},
							}},
						},
					},
				}},
			},
		}

		port443 := gatewayv1.PortNumber(443)
		httpRouteKey := types.NamespacedName{Namespace: "default", Name: "test-ingress-example-com"}

		ir := &providerir.ProviderIR{
			Gateways: make(map[types.NamespacedName]providerir.GatewayContext),
			HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
				httpRouteKey: {
					HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ingress-example-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							Rules: []gatewayv1.HTTPRouteRule{{
								BackendRefs: []gatewayv1.HTTPBackendRef{{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "my-service",
											Port: &port443,
										},
									},
								}},
							}},
						},
					},
					RuleBackendSources: [][]providerir.BackendSource{
						{{Ingress: &ingress}},
					},
				},
			},
			TLSRoutes: make(map[types.NamespacedName]gatewayv1.TLSRoute),
			Services:  make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
		}

		errs := sslPassthroughFeature(testNotify, []networkingv1.Ingress{ingress}, nil, ir)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}

		// HTTPRoute should still exist.
		if _, exists := ir.HTTPRoutes[httpRouteKey]; !exists {
			t.Fatal("HTTPRoute should not be removed when ssl-passthrough is false")
		}

		// No TLSRoute should be created.
		if len(ir.TLSRoutes) != 0 {
			t.Fatalf("expected 0 TLSRoutes, got %d", len(ir.TLSRoutes))
		}
	})

	t.Run("no ssl-passthrough annotation is no-op", func(t *testing.T) {
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: "default",
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "example.com",
				}},
			},
		}

		port80 := gatewayv1.PortNumber(80)
		httpRouteKey := types.NamespacedName{Namespace: "default", Name: "test-ingress-example-com"}

		ir := &providerir.ProviderIR{
			Gateways: make(map[types.NamespacedName]providerir.GatewayContext),
			HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
				httpRouteKey: {
					HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ingress-example-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							Rules: []gatewayv1.HTTPRouteRule{{
								BackendRefs: []gatewayv1.HTTPBackendRef{{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "my-service",
											Port: &port80,
										},
									},
								}},
							}},
						},
					},
					RuleBackendSources: [][]providerir.BackendSource{
						{{Ingress: &ingress}},
					},
				},
			},
			TLSRoutes: make(map[types.NamespacedName]gatewayv1.TLSRoute),
			Services:  make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
		}

		errs := sslPassthroughFeature(testNotify, []networkingv1.Ingress{ingress}, nil, ir)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}

		if _, exists := ir.HTTPRoutes[httpRouteKey]; !exists {
			t.Fatal("HTTPRoute should remain when ssl-passthrough annotation is absent")
		}
		if len(ir.TLSRoutes) != 0 {
			t.Fatalf("expected 0 TLSRoutes, got %d", len(ir.TLSRoutes))
		}
	})

	t.Run("mixed passthrough and non-passthrough on same host: passthrough wins", func(t *testing.T) {
		passthroughIngress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "passthrough-ingress",
				Namespace: "default",
				Annotations: map[string]string{
					SSLPassthroughAnnotation: "true",
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("nginx"),
				Rules: []networkingv1.IngressRule{{
					Host: "example.com",
				}},
			},
		}
		normalIngress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "normal-ingress",
				Namespace: "default",
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("nginx"),
				Rules: []networkingv1.IngressRule{{
					Host: "example.com",
				}},
			},
		}

		port443 := gatewayv1.PortNumber(443)
		port80 := gatewayv1.PortNumber(80)
		httpRouteKey := types.NamespacedName{Namespace: "default", Name: "mixed-route"}
		gwKey := types.NamespacedName{Namespace: "default", Name: "nginx"}

		ir := &providerir.ProviderIR{
			Gateways: map[types.NamespacedName]providerir.GatewayContext{
				gwKey: {
					Gateway: gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "nginx",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "nginx",
							Listeners:        []gatewayv1.Listener{},
						},
					},
				},
			},
			HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
				httpRouteKey: {
					HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mixed-route",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{Name: "nginx"}},
							},
							Hostnames: []gatewayv1.Hostname{"example.com"},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									// Backend from the passthrough ingress.
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "passthrough-svc",
												Port: &port443,
											},
										},
									}},
								},
								{
									// Backend from the normal ingress.
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "normal-svc",
												Port: &port80,
											},
										},
									}},
								},
							},
						},
					},
					RuleBackendSources: [][]providerir.BackendSource{
						{{Ingress: &passthroughIngress}},
						{{Ingress: &normalIngress}},
					},
				},
			},
			TLSRoutes: make(map[types.NamespacedName]gatewayv1.TLSRoute),
			Services:  make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
		}

		errs := sslPassthroughFeature(testNotify, []networkingv1.Ingress{passthroughIngress, normalIngress}, nil, ir)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}

		// HTTPRoute should be preserved with only the non-passthrough backend.
		// The passthrough backend is moved to the TLSRoute, but the normal
		// ingress's HTTP rules remain for port 80 traffic.
		httpRoute, exists := ir.HTTPRoutes[httpRouteKey]
		if !exists {
			t.Fatal("HTTPRoute should be preserved with non-passthrough rules")
		}
		if len(httpRoute.HTTPRoute.Spec.Rules) != 1 {
			t.Fatalf("expected 1 remaining HTTPRoute rule, got %d", len(httpRoute.HTTPRoute.Spec.Rules))
		}
		if len(httpRoute.HTTPRoute.Spec.Rules[0].BackendRefs) != 1 {
			t.Fatalf("expected 1 backend ref in remaining rule, got %d", len(httpRoute.HTTPRoute.Spec.Rules[0].BackendRefs))
		}
		if httpRoute.HTTPRoute.Spec.Rules[0].BackendRefs[0].Name != "normal-svc" {
			t.Errorf("expected remaining backend normal-svc, got %s", httpRoute.HTTPRoute.Spec.Rules[0].BackendRefs[0].Name)
		}

		// TLSRoute should be created with only the passthrough backend.
		tlsRouteKey := types.NamespacedName{Namespace: "default", Name: "mixed-route-tls-passthrough"}
		tlsRoute, exists := ir.TLSRoutes[tlsRouteKey]
		if !exists {
			t.Fatalf("expected TLSRoute %v to be created", tlsRouteKey)
		}

		if len(tlsRoute.Spec.Rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(tlsRoute.Spec.Rules))
		}
		if len(tlsRoute.Spec.Rules[0].BackendRefs) != 1 {
			t.Fatalf("expected 1 backend ref (passthrough only), got %d", len(tlsRoute.Spec.Rules[0].BackendRefs))
		}
		if tlsRoute.Spec.Rules[0].BackendRefs[0].Name != "passthrough-svc" {
			t.Errorf("expected backend name passthrough-svc, got %s", tlsRoute.Spec.Rules[0].BackendRefs[0].Name)
		}
		if *tlsRoute.Spec.Rules[0].BackendRefs[0].Port != 443 {
			t.Errorf("expected backend port 443, got %d", *tlsRoute.Spec.Rules[0].BackendRefs[0].Port)
		}
	})

	t.Run("multiple backends are deduplicated in TLSRoute", func(t *testing.T) {
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-path",
				Namespace: "default",
				Annotations: map[string]string{
					SSLPassthroughAnnotation: "true",
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("nginx"),
				Rules: []networkingv1.IngressRule{{
					Host: "multi.example.com",
				}},
			},
		}

		port443 := gatewayv1.PortNumber(443)
		port8443 := gatewayv1.PortNumber(8443)
		httpRouteKey := types.NamespacedName{Namespace: "default", Name: "multi-path-multi-example-com"}
		gwKey := types.NamespacedName{Namespace: "default", Name: "nginx"}

		ir := &providerir.ProviderIR{
			Gateways: map[types.NamespacedName]providerir.GatewayContext{
				gwKey: {
					Gateway: gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "nginx",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "nginx",
							Listeners:        []gatewayv1.Listener{},
						},
					},
				},
			},
			HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
				httpRouteKey: {
					HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "multi-path-multi-example-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{Name: "nginx"}},
							},
							Hostnames: []gatewayv1.Hostname{"multi.example.com"},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "svc-a",
												Port: &port443,
											},
										},
									}},
								},
								{
									BackendRefs: []gatewayv1.HTTPBackendRef{
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "svc-a",
													Port: &port443,
												},
											},
										},
										{
											BackendRef: gatewayv1.BackendRef{
												BackendObjectReference: gatewayv1.BackendObjectReference{
													Name: "svc-b",
													Port: &port8443,
												},
											},
										},
									},
								},
							},
						},
					},
					RuleBackendSources: [][]providerir.BackendSource{
						{{Ingress: &ingress}},
						{{Ingress: &ingress}, {Ingress: &ingress}},
					},
				},
			},
			TLSRoutes: make(map[types.NamespacedName]gatewayv1.TLSRoute),
			Services:  make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
		}

		errs := sslPassthroughFeature(testNotify, []networkingv1.Ingress{ingress}, nil, ir)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}

		// HTTPRoute should be removed.
		if _, exists := ir.HTTPRoutes[httpRouteKey]; exists {
			t.Fatal("expected HTTPRoute to be removed")
		}

		tlsRouteKey := types.NamespacedName{Namespace: "default", Name: "multi-path-multi-example-com-tls-passthrough"}
		tlsRoute, exists := ir.TLSRoutes[tlsRouteKey]
		if !exists {
			t.Fatalf("expected TLSRoute %v to be created", tlsRouteKey)
		}

		// Should have 2 unique backends (svc-a:443, svc-b:8443), not 3.
		if len(tlsRoute.Spec.Rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(tlsRoute.Spec.Rules))
		}
		if len(tlsRoute.Spec.Rules[0].BackendRefs) != 2 {
			t.Fatalf("expected 2 deduplicated backend refs, got %d", len(tlsRoute.Spec.Rules[0].BackendRefs))
		}
	})

	t.Run("GVK is set correctly on TLSRoute", func(t *testing.T) {
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gvk-test",
				Namespace: "default",
				Annotations: map[string]string{
					SSLPassthroughAnnotation: "true",
				},
			},
		}

		port443 := gatewayv1.PortNumber(443)
		httpRouteKey := types.NamespacedName{Namespace: "default", Name: "gvk-route"}
		gwKey := types.NamespacedName{Namespace: "default", Name: "nginx"}

		ir := &providerir.ProviderIR{
			Gateways: map[types.NamespacedName]providerir.GatewayContext{
				gwKey: {
					Gateway: gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
						Spec:       gatewayv1.GatewaySpec{GatewayClassName: "nginx"},
					},
				},
			},
			HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
				httpRouteKey: {
					HTTPRoute: gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{Name: "gvk-route", Namespace: "default"},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{Name: "nginx"}},
							},
							Hostnames: []gatewayv1.Hostname{"gvk.example.com"},
							Rules: []gatewayv1.HTTPRouteRule{{
								BackendRefs: []gatewayv1.HTTPBackendRef{{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "svc",
											Port: &port443,
										},
									},
								}},
							}},
						},
					},
					RuleBackendSources: [][]providerir.BackendSource{
						{{Ingress: &ingress}},
					},
				},
			},
			TLSRoutes: make(map[types.NamespacedName]gatewayv1.TLSRoute),
			Services:  make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
		}

		_ = sslPassthroughFeature(testNotify, []networkingv1.Ingress{ingress}, nil, ir)

		tlsRouteKey := types.NamespacedName{Namespace: "default", Name: "gvk-route-tls-passthrough"}
		tlsRoute, exists := ir.TLSRoutes[tlsRouteKey]
		if !exists {
			t.Fatalf("expected TLSRoute to be created")
		}

		expectedGVK := common.TLSRouteGVK
		gotGVK := tlsRoute.GroupVersionKind()
		if !apiequality.Semantic.DeepEqual(gotGVK, expectedGVK) {
			t.Errorf("GVK mismatch:\n%s", cmp.Diff(expectedGVK, gotGVK))
		}
	})
}
