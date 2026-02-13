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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestApplyTrailingSlashPathRedirectsToEmitterIR(t *testing.T) {
	prefix := gatewayv1.PathMatchPathPrefix
	exact := gatewayv1.PathMatchExact
	key := types.NamespacedName{Namespace: "default", Name: "route"}

	ir := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {
				HTTPRoute: gatewayv1.HTTPRoute{
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
				},
			},
		},
	}

	applyTrailingSlashPathRedirectsToEmitterIR(&ir)

	got := ir.HTTPRoutes[key].Spec.Rules
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

	// If an exact /foo rule exists, no redirect rule should be added.
	ir2 := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {
				HTTPRoute: gatewayv1.HTTPRoute{
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
				},
			},
		},
	}
	applyTrailingSlashPathRedirectsToEmitterIR(&ir2)
	if len(ir2.HTTPRoutes[key].Spec.Rules) != 1 {
		t.Fatalf("expected no extra redirect rule when exact /foo exists, got %d rules", len(ir2.HTTPRoutes[key].Spec.Rules))
	}
}
