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
	"reflect"
	"testing"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestApplyRewriteTargetToEmitterIR_SetsRewriteHeadersAndRegex(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "ing",
			Annotations: map[string]string{
				RewriteTargetAnnotation:    "/rewritten",
				XForwardedPrefixAnnotation: "/prefix",
				UseRegexAnnotation:         "true",
			},
		},
	}

	pIR := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				RuleBackendSources: [][]providerir.BackendSource{{{Ingress: &ing}}},
			},
		},
	}

	eIR := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {
				HTTPRoute: gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{{}}}},
			},
		},
	}

	applyRewriteTargetToEmitterIR(pIR, &eIR)

	got := eIR.HTTPRoutes[key].PathRewriteByRuleIdx[0]
	if got == nil {
		t.Fatalf("expected PathRewriteByRuleIdx[0] to be set")
	}
	if got.ReplaceFullPath != "/rewritten" {
		t.Fatalf("expected ReplaceFullPath=/rewritten, got %q", got.ReplaceFullPath)
	}
	if got.Regex != true {
		t.Fatalf("expected Regex=true, got %v", got.Regex)
	}
	wantHeaders := map[string]string{"X-Forwarded-Prefix": "/prefix"}
	if !reflect.DeepEqual(got.Headers, wantHeaders) {
		t.Fatalf("expected headers %v, got %v", wantHeaders, got.Headers)
	}
}

func TestApplyRewriteTargetToEmitterIR_PrefersNonCanaryIngressSource(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	canary := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "canary",
			Annotations: map[string]string{
				CanaryAnnotation:        "true",
				RewriteTargetAnnotation: "/bad",
			},
		},
	}
	main := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "main",
			Annotations: map[string]string{
				RewriteTargetAnnotation:    "/good",
				XForwardedPrefixAnnotation: "/p",
			},
		},
	}

	pIR := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				RuleBackendSources: [][]providerir.BackendSource{{
					{Ingress: &canary},
					{Ingress: &main},
				}},
			},
		},
	}

	eIR := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {
				HTTPRoute: gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{{}}}},
			},
		},
	}

	applyRewriteTargetToEmitterIR(pIR, &eIR)

	got := eIR.HTTPRoutes[key].PathRewriteByRuleIdx[0]
	if got == nil {
		t.Fatalf("expected PathRewriteByRuleIdx[0] to be set")
	}
	if got.ReplaceFullPath != "/good" {
		t.Fatalf("expected ReplaceFullPath=/good, got %q", got.ReplaceFullPath)
	}
	wantHeaders := map[string]string{"X-Forwarded-Prefix": "/p"}
	if !reflect.DeepEqual(got.Headers, wantHeaders) {
		t.Fatalf("expected headers %v, got %v", wantHeaders, got.Headers)
	}
}
