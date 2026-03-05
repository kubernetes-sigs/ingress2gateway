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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestApplyTrailingSlashPathRedirectsToEmitterIR(t *testing.T) {
	prefix := gatewayv1.PathMatchPathPrefix
	exact := gatewayv1.PathMatchExact
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	baseRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{{
						Path: &gatewayv1.HTTPPathMatch{
							Type:  &prefix,
							Value: ptr.To("/foo/"),
						},
					}},
				},
			},
		},
	}

	eIR := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {HTTPRoute: *baseRoute.DeepCopy()},
		},
	}
	pIR := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				HTTPRoute:          *baseRoute.DeepCopy(),
				RuleBackendSources: [][]providerir.BackendSource{{}},
			},
		},
	}

	applyTrailingSlashPathRedirectsToEmitterIR(&pIR, &eIR)

	got := eIR.HTTPRoutes[key].Spec.Rules
	if len(got) != 2 {
		t.Fatalf("expected 2 rules (original + redirect), got %d", len(got))
	}

	redirect := got[1]
	if len(redirect.Matches) != 1 || redirect.Matches[0].Path == nil || redirect.Matches[0].Path.Value == nil || *redirect.Matches[0].Path.Value != "/foo" {
		t.Fatalf("expected redirect match on /foo, got %#v", redirect.Matches)
	}
	if redirect.Matches[0].Path.Type == nil || *redirect.Matches[0].Path.Type != gatewayv1.PathMatchExact {
		t.Fatalf("expected redirect match type Exact, got %#v", redirect.Matches[0].Path.Type)
	}
	if len(redirect.Filters) != 1 || redirect.Filters[0].RequestRedirect == nil {
		t.Fatalf("expected a single RequestRedirect filter, got %#v", redirect.Filters)
	}
	if redirect.Filters[0].RequestRedirect.StatusCode == nil || *redirect.Filters[0].RequestRedirect.StatusCode != 301 {
		t.Fatalf("expected redirect status code 301, got %#v", redirect.Filters[0].RequestRedirect.StatusCode)
	}
	if redirect.Filters[0].RequestRedirect.Path == nil || redirect.Filters[0].RequestRedirect.Path.ReplaceFullPath == nil || *redirect.Filters[0].RequestRedirect.Path.ReplaceFullPath != "/foo/" {
		t.Fatalf("expected redirect target /foo/, got %#v", redirect.Filters[0].RequestRedirect.Path)
	}

	// Verify ProviderIR RuleBackendSources stays in sync.
	pSources := pIR.HTTPRoutes[key].RuleBackendSources
	if len(pSources) != len(got) {
		t.Fatalf("expected RuleBackendSources length %d to match rules length %d", len(pSources), len(got))
	}
	if len(pSources[1]) != 0 {
		t.Fatalf("expected empty BackendSource slice for redirect rule, got %d entries", len(pSources[1]))
	}

	// If an exact /foo rule exists, no redirect rule should be added.
	noRedirectRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &exact,
								Value: ptr.To("/foo"),
							},
						},
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &prefix,
								Value: ptr.To("/foo/"),
							},
						},
					},
				},
			},
		},
	}
	eIR2 := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {HTTPRoute: *noRedirectRoute.DeepCopy()},
		},
	}
	pIR2 := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				HTTPRoute:          *noRedirectRoute.DeepCopy(),
				RuleBackendSources: [][]providerir.BackendSource{{}},
			},
		},
	}
	applyTrailingSlashPathRedirectsToEmitterIR(&pIR2, &eIR2)
	if len(eIR2.HTTPRoutes[key].Spec.Rules) != 1 {
		t.Fatalf("expected no extra redirect rule when exact /foo exists, got %d rules", len(eIR2.HTTPRoutes[key].Spec.Rules))
	}
	if len(pIR2.HTTPRoutes[key].RuleBackendSources) != 1 {
		t.Fatalf("expected RuleBackendSources length 1 (unchanged), got %d", len(pIR2.HTTPRoutes[key].RuleBackendSources))
	}

	// If a prefix /foo rule exists, no redirect rule should be added either.
	noRedirectRoute2 := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &prefix,
								Value: ptr.To("/foo"),
							},
						},
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &prefix,
								Value: ptr.To("/foo/"),
							},
						},
					},
				},
			},
		},
	}
	eIR3 := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {HTTPRoute: *noRedirectRoute2.DeepCopy()},
		},
	}
	pIR3 := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				HTTPRoute:          *noRedirectRoute2.DeepCopy(),
				RuleBackendSources: [][]providerir.BackendSource{{}},
			},
		},
	}
	applyTrailingSlashPathRedirectsToEmitterIR(&pIR3, &eIR3)
	if len(eIR3.HTTPRoutes[key].Spec.Rules) != 1 {
		t.Fatalf("expected no extra redirect rule when prefix /foo exists, got %d rules", len(eIR3.HTTPRoutes[key].Spec.Rules))
	}
	if len(pIR3.HTTPRoutes[key].RuleBackendSources) != 1 {
		t.Fatalf("expected RuleBackendSources length 1 (unchanged), got %d", len(pIR3.HTTPRoutes[key].RuleBackendSources))
	}

	// A shorter PathPrefix that covers the redirect source should suppress the redirect.
	// E.g., PathPrefix "/a" covers "/a/b/c", so no redirect from "/a/b/c" to "/a/b/c/" should be added.
	prefixCoverRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &prefix,
								Value: ptr.To("/a"),
							},
						},
					},
				},
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &exact,
								Value: ptr.To("/a/b/c/"),
							},
						},
					},
				},
			},
		},
	}
	eIR4 := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {HTTPRoute: *prefixCoverRoute.DeepCopy()},
		},
	}
	pIR4 := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				HTTPRoute:          *prefixCoverRoute.DeepCopy(),
				RuleBackendSources: [][]providerir.BackendSource{{}, {}},
			},
		},
	}
	applyTrailingSlashPathRedirectsToEmitterIR(&pIR4, &eIR4)
	if len(eIR4.HTTPRoutes[key].Spec.Rules) != 2 {
		t.Fatalf("expected no redirect when PathPrefix /a covers /a/b/c, got %d rules", len(eIR4.HTTPRoutes[key].Spec.Rules))
	}

	// PathPrefix "/" covers everything, so no redirect should be added.
	rootPrefixRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &prefix,
								Value: ptr.To("/"),
							},
						},
					},
				},
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &prefix,
								Value: ptr.To("/xyz/"),
							},
						},
					},
				},
			},
		},
	}
	eIR5 := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {HTTPRoute: *rootPrefixRoute.DeepCopy()},
		},
	}
	pIR5 := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				HTTPRoute:          *rootPrefixRoute.DeepCopy(),
				RuleBackendSources: [][]providerir.BackendSource{{}, {}},
			},
		},
	}
	applyTrailingSlashPathRedirectsToEmitterIR(&pIR5, &eIR5)
	if len(eIR5.HTTPRoutes[key].Spec.Rules) != 2 {
		t.Fatalf("expected no redirect when PathPrefix / covers /xyz, got %d rules", len(eIR5.HTTPRoutes[key].Spec.Rules))
	}

	// PathPrefix "/ab" should NOT cover "/a/b/c" (not a segment boundary match).
	nonSegmentRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &prefix,
								Value: ptr.To("/ab"),
							},
						},
					},
				},
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &exact,
								Value: ptr.To("/a/b/c/"),
							},
						},
					},
				},
			},
		},
	}
	eIR6 := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {HTTPRoute: *nonSegmentRoute.DeepCopy()},
		},
	}
	pIR6 := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{
			key: {
				HTTPRoute:          *nonSegmentRoute.DeepCopy(),
				RuleBackendSources: [][]providerir.BackendSource{{}, {}},
			},
		},
	}
	applyTrailingSlashPathRedirectsToEmitterIR(&pIR6, &eIR6)
	if len(eIR6.HTTPRoutes[key].Spec.Rules) != 3 {
		t.Fatalf("expected redirect for /a/b/c when /ab does NOT cover it (non-segment boundary), got %d rules", len(eIR6.HTTPRoutes[key].Spec.Rules))
	}

	// Routes only in EmitterIR (e.g. SSL redirect routes) should not create phantom ProviderIR entries.
	sslOnlyKey := types.NamespacedName{Namespace: "default", Name: "route-ssl-redirect"}
	sslRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: sslOnlyKey.Name, Namespace: sslOnlyKey.Namespace},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
								Scheme: ptr.To("https"),
							},
						},
					},
				},
			},
		},
	}
	eIR7 := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			sslOnlyKey: {HTTPRoute: *sslRoute.DeepCopy()},
		},
	}
	pIR7 := providerir.ProviderIR{
		HTTPRoutes: map[types.NamespacedName]providerir.HTTPRouteContext{},
	}
	applyTrailingSlashPathRedirectsToEmitterIR(&pIR7, &eIR7)
	if _, exists := pIR7.HTTPRoutes[sslOnlyKey]; exists {
		t.Fatalf("expected no phantom ProviderIR entry for SSL-only route, but one was created")
	}
}
