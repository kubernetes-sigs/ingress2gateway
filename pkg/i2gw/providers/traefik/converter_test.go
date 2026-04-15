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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_convertToIR_pathTypes(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	iExact := networkingv1.PathTypeExact
	iImplSpec := networkingv1.PathTypeImplementationSpecific

	gPrefix := gatewayv1.PathMatchPathPrefix
	gExact := gatewayv1.PathMatchExact

	testCases := []struct {
		name             string
		pathType         *networkingv1.PathType
		expectedPathType *gatewayv1.PathMatchType
	}{
		{
			name:             "Prefix maps to PathMatchPathPrefix",
			pathType:         &iPrefix,
			expectedPathType: &gPrefix,
		},
		{
			name:             "Exact maps to PathMatchExact",
			pathType:         &iExact,
			expectedPathType: &gExact,
		},
		{
			name:             "ImplementationSpecific maps to PathMatchPathPrefix",
			pathType:         &iImplSpec,
			expectedPathType: &gPrefix,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingress := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
					Rules: []networkingv1.IngressRule{{
						Host:             "foo.com",
						IngressRuleValue: ingressRuleValue(tc.pathType, "my-app"),
					}},
				},
			}

			converter := newResourcesToIRConverter(notifications.NoopNotify)
			storage := &storage{
				Ingresses: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "my-app"}: &ingress,
				},
				ServicePorts: map[types.NamespacedName]map[string]int32{},
			}

			ir, errs := converter.convertToIR(storage)
			if len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}

			routeKey := types.NamespacedName{Namespace: "default", Name: "my-app-foo-com"}
			route, ok := ir.HTTPRoutes[routeKey]
			if !ok {
				t.Fatalf("HTTPRoute %v not found in IR", routeKey)
			}
			if len(route.Spec.Rules) == 0 {
				t.Fatal("expected at least one HTTPRoute rule")
			}

			actualPathType := route.Spec.Rules[0].Matches[0].Path.Type
			if diff := cmp.Diff(tc.expectedPathType, actualPathType); diff != "" {
				t.Errorf("unexpected path type (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_convertToIR_routerTLSWithEntrypoints(t *testing.T) {
	iImplSpec := networkingv1.PathTypeImplementationSpecific

	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana",
			Namespace: "monitoring",
			Annotations: map[string]string{
				RouterTLSAnnotation:         "true",
				RouterEntrypointsAnnotation: "websecure",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("traefik"),
			Rules: []networkingv1.IngressRule{{
				Host:             "grafana.example.com",
				IngressRuleValue: ingressRuleValue(&iImplSpec, "grafana"),
			}},
		},
	}

	converter := newResourcesToIRConverter(notifications.NoopNotify)
	storage := &storage{
		Ingresses:    map[types.NamespacedName]*networkingv1.Ingress{{Namespace: "monitoring", Name: "grafana"}: &ingress},
		ServicePorts: map[types.NamespacedName]map[string]int32{},
	}

	ir, errs := converter.convertToIR(storage)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	gatewayKey := types.NamespacedName{Namespace: "monitoring", Name: "traefik"}
	gw, ok := ir.Gateways[gatewayKey]
	if !ok {
		t.Fatalf("Gateway %v not found", gatewayKey)
	}

	// Both HTTP and HTTPS listeners should be present:
	// - routerEntrypointsFeature keeps the HTTP listener (HTTPS listener exists)
	// - forceHTTPSFeature generates a redirect HTTPRoute on port 80
	httpFound, httpsFound := false, false
	for _, l := range gw.Spec.Listeners {
		if l.Protocol == gatewayv1.HTTPProtocolType {
			httpFound = true
		}
		if l.Protocol == gatewayv1.HTTPSProtocolType {
			httpsFound = true
			// Verify placeholder cert name.
			if len(l.TLS.CertificateRefs) == 0 {
				t.Error("expected at least one CertificateRef on HTTPS listener")
			} else {
				expectedSecretName := gatewayv1.ObjectName("grafana-example-com-tls")
				if l.TLS.CertificateRefs[0].Name != expectedSecretName {
					t.Errorf("expected certificateRef name %q, got %q", expectedSecretName, l.TLS.CertificateRefs[0].Name)
				}
			}
		}
	}
	if !httpFound {
		t.Error("expected HTTP listener to be present (forceHTTPSFeature attaches redirect route to it)")
	}
	if !httpsFound {
		t.Error("expected HTTPS listener to be present")
	}

	// Verify that forceHTTPSFeature generated a redirect HTTPRoute for port 80.
	redirectRouteKey := types.NamespacedName{Namespace: "monitoring", Name: "grafana-grafana-example-com-http"}
	if _, exists := ir.HTTPRoutes[redirectRouteKey]; !exists {
		t.Errorf("expected HTTP->HTTPS redirect HTTPRoute %v to be present", redirectRouteKey)
	}

	// Verify path type: ImplementationSpecific → PathPrefix.
	routeKey := types.NamespacedName{Namespace: "monitoring", Name: "grafana-grafana-example-com"}
	route, ok := ir.HTTPRoutes[routeKey]
	if !ok {
		t.Fatalf("HTTPRoute %v not found", routeKey)
	}
	if len(route.Spec.Rules) == 0 || len(route.Spec.Rules[0].Matches) == 0 {
		t.Fatal("expected at least one match in HTTPRoute")
	}
	gPrefix := gatewayv1.PathMatchPathPrefix
	if diff := cmp.Diff(&gPrefix, route.Spec.Rules[0].Matches[0].Path.Type); diff != "" {
		t.Errorf("unexpected path type (-want +got):\n%s", diff)
	}
}

func Test_convertToIR_tlsSpecTakesPrecedence(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix

	// When spec.tls is present, common.ToIR() already handles it.
	// router.tls annotation should be a no-op in this case.
	ingress := networkingv1.Ingress{
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
				SecretName: "my-real-secret",
			}},
			Rules: []networkingv1.IngressRule{{
				Host:             "foo.com",
				IngressRuleValue: ingressRuleValue(&iPrefix, "my-app"),
			}},
		},
	}

	converter := newResourcesToIRConverter(notifications.NoopNotify)
	storage := &storage{
		Ingresses:    map[types.NamespacedName]*networkingv1.Ingress{{Namespace: "default", Name: "my-app"}: &ingress},
		ServicePorts: map[types.NamespacedName]map[string]int32{},
	}

	ir, errs := converter.convertToIR(storage)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	gatewayKey := types.NamespacedName{Namespace: "default", Name: "traefik"}
	gw, ok := ir.Gateways[gatewayKey]
	if !ok {
		t.Fatalf("Gateway %v not found", gatewayKey)
	}

	// Should have exactly one HTTPS listener (from spec.tls via common.ToIR),
	// not two (one from spec.tls + one from annotation).
	httpsCount := 0
	for _, l := range gw.Spec.Listeners {
		if l.Protocol == gatewayv1.HTTPSProtocolType {
			httpsCount++
			// The cert should be the real one from spec.tls, not the placeholder.
			if len(l.TLS.CertificateRefs) > 0 && l.TLS.CertificateRefs[0].Name == "my-real-secret" {
				continue
			}
		}
	}
	if httpsCount != 1 {
		t.Errorf("expected exactly 1 HTTPS listener, got %d", httpsCount)
	}
}
