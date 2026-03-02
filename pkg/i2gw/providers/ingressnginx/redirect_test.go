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
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestAddDefaultSSLRedirect_enabled(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	parentRefs := []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing",
			Annotations: map[string]string{
				// no SSLRedirectAnnotation -> default enabled
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret"}},
		},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: append([]gatewayv1.ParentReference(nil), parentRefs...),
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{}}},
			},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect(&pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route %v to be added", redirectKey)
	}

	if len(redirectCtx.Spec.ParentRefs) != 1 || redirectCtx.Spec.ParentRefs[0].Port == nil || *redirectCtx.Spec.ParentRefs[0].Port != 80 {
		t.Fatalf("expected redirect route parentRef port 80, got %#v", redirectCtx.Spec.ParentRefs)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 || origCtx.Spec.ParentRefs[0].Port == nil || *origCtx.Spec.ParentRefs[0].Port != 443 {
		t.Fatalf("expected original route parentRef port 443, got %#v", origCtx.Spec.ParentRefs)
	}

	if len(redirectCtx.Spec.Rules) != 1 || len(redirectCtx.Spec.Rules[0].Filters) != 1 {
		t.Fatalf("expected redirect route to have 1 rule with 1 filter, got %#v", redirectCtx.Spec.Rules)
	}

	f := redirectCtx.Spec.Rules[0].Filters[0]
	if f.Type != gatewayv1.HTTPRouteFilterRequestRedirect || f.RequestRedirect == nil {
		t.Fatalf("expected RequestRedirect filter, got %#v", f)
	}
	if f.RequestRedirect.Scheme == nil || *f.RequestRedirect.Scheme != "https" {
		t.Fatalf("expected scheme https, got %#v", f.RequestRedirect.Scheme)
	}
	if f.RequestRedirect.StatusCode == nil || *f.RequestRedirect.StatusCode != 308 {
		t.Fatalf("expected status code 308, got %#v", f.RequestRedirect.StatusCode)
	}
}

func TestAddDefaultSSLRedirect_disabledByAnnotation(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret"}},
		},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect(&pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	if _, ok := eIR.HTTPRoutes[redirectKey]; ok {
		t.Fatalf("did not expect redirect route %v to be added", redirectKey)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parentRef, got %#v", origCtx.Spec.ParentRefs)
	}
	if origCtx.Spec.ParentRefs[0].Port != nil {
		t.Fatalf("expected original route parentRef port to remain nil, got %#v", origCtx.Spec.ParentRefs[0].Port)
	}
}

func TestAddDefaultSSLRedirect_conflictingAnnotations(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}
	parentRefs := []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}}

	ingEnabled := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-enabled",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret"}},
		},
	}

	ingDisabled := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "ing-disabled",
			Annotations: map[string]string{
				SSLRedirectAnnotation: "false",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "secret"}},
		},
	}

	pathA := "/a"
	pathB := "/b"
	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: append([]gatewayv1.ParentReference(nil), parentRefs...),
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathA}}}},
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Value: &pathB}}}},
			},
		},
	}

	// Two rules, each from a different ingress with conflicting ssl-redirect values.
	// Per-rule semantics: only the rule from ingEnabled should get a redirect.
	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{
			{{Ingress: &ingEnabled}},
			{{Ingress: &ingDisabled}},
		},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect(&pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	redirectCtx, ok := eIR.HTTPRoutes[redirectKey]
	if !ok {
		t.Fatalf("expected redirect route %v to be created for the enabled rule", redirectKey)
	}

	// Only one redirect rule (for /a), not two.
	if len(redirectCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 redirect rule, got %d", len(redirectCtx.Spec.Rules))
	}

	if len(redirectCtx.Spec.Rules[0].Matches) != 1 || *redirectCtx.Spec.Rules[0].Matches[0].Path.Value != "/a" {
		t.Fatalf("expected redirect rule to match /a, got %#v", redirectCtx.Spec.Rules[0].Matches)
	}

	// A passthrough route should be created on port 80 for /b.
	httpKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-http"}
	httpCtx, ok := eIR.HTTPRoutes[httpKey]
	if !ok {
		t.Fatalf("expected passthrough route %v to be created for non-redirect paths", httpKey)
	}
	if len(httpCtx.Spec.Rules) != 1 {
		t.Fatalf("expected 1 passthrough rule, got %d", len(httpCtx.Spec.Rules))
	}
	if len(httpCtx.Spec.Rules[0].Matches) != 1 || *httpCtx.Spec.Rules[0].Matches[0].Path.Value != "/b" {
		t.Fatalf("expected passthrough rule to match /b, got %#v", httpCtx.Spec.Rules[0].Matches)
	}
	if len(httpCtx.Spec.ParentRefs) != 1 || httpCtx.Spec.ParentRefs[0].Port == nil || *httpCtx.Spec.ParentRefs[0].Port != 80 {
		t.Fatalf("expected passthrough route parentRef port 80, got %#v", httpCtx.Spec.ParentRefs)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 || origCtx.Spec.ParentRefs[0].Port == nil || *origCtx.Spec.ParentRefs[0].Port != 443 {
		t.Fatalf("expected original route parentRef port 443, got %#v", origCtx.Spec.ParentRefs)
	}
}

func TestAddDefaultSSLRedirect_noTLS(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        "ing",
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{},
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName("gw")}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
		},
	}

	pIR := providerir.ProviderIR{HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{}}
	pIR.HTTPRoutes[key] = providerir.HTTPRouteContext{
		HTTPRoute: route,
		RuleBackendSources: [][]providerir.BackendSource{{
			{Ingress: &ing},
		}},
	}

	eIR := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{}}
	eIR.HTTPRoutes[key] = emitterir.HTTPRouteContext{HTTPRoute: route}

	addDefaultSSLRedirect(&pIR, &eIR)

	redirectKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Name + "-ssl-redirect"}
	if _, ok := eIR.HTTPRoutes[redirectKey]; ok {
		t.Fatalf("did not expect redirect route %v to be added", redirectKey)
	}

	origCtx := eIR.HTTPRoutes[key]
	if len(origCtx.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parentRef, got %#v", origCtx.Spec.ParentRefs)
	}
	if origCtx.Spec.ParentRefs[0].Port != nil {
		t.Fatalf("expected original route parentRef port to remain nil, got %#v", origCtx.Spec.ParentRefs[0].Port)
	}
}
