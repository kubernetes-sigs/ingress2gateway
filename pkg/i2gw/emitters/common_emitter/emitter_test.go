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

package common_emitter

import (
	"testing"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestApplyHTTPRouteRequestTimeouts(t *testing.T) {
	d := gatewayv1.Duration("10s")

	testCases := []struct {
		name    string
		ctx     emitterir.HTTPRouteContext
		wantSet bool
		wantErr bool
	}{
		{
			name: "sets request timeout",
			ctx: emitterir.HTTPRouteContext{
				HTTPRoute:       gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{{}}}},
				RequestTimeouts: map[int]*gatewayv1.Duration{0: &d},
			},
			wantSet: true,
		},
		{
			name: "nil duration ignored",
			ctx: emitterir.HTTPRouteContext{
				HTTPRoute:       gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{{}}}},
				RequestTimeouts: map[int]*gatewayv1.Duration{0: nil},
			},
			wantSet: false,
		},
		{
			name: "out of range rule index",
			ctx: emitterir.HTTPRouteContext{
				HTTPRoute:       gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{{}}}},
				RequestTimeouts: map[int]*gatewayv1.Duration{1: &d},
			},
			wantErr: true,
		},
	}

	key := types.NamespacedName{Namespace: "ns", Name: "route"}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ir := emitterir.EmitterIR{HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{key: tc.ctx}}
			errList := applyHTTPRouteRequestTimeouts(&ir)

			gotCtx := ir.HTTPRoutes[key]
			if gotCtx.RequestTimeouts != nil {
				t.Fatalf("expected RequestTimeouts to be nil after apply")
			}
			if tc.wantErr {
				if len(errList) == 0 {
					t.Fatalf("expected error")
				}
				return
			}
			if len(errList) > 0 {
				t.Fatalf("expected no errors, got %v", errList)
			}

			got := gotCtx.Spec.Rules[0].Timeouts
			if tc.wantSet {
				if got == nil || got.Request == nil {
					t.Fatalf("expected request timeout to be set")
				}
				if *got.Request != d {
					t.Fatalf("expected %v, got %v", d, *got.Request)
				}
				return
			}

			if got != nil {
				t.Fatalf("expected timeouts to be nil, got %v", got)
			}
		})
	}
}

func TestEmitter_Emit_appliesPathRewriteReplaceFullPath(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns", Name: "route"}

	ir := emitterir.EmitterIR{
		HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
			key: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{{}},
					},
				},
				PathRewriteByRuleIdx: map[int]*emitterir.PathRewrite{
					0: {ReplaceFullPath: "/foo"},
				},
			},
		},
	}

	e := NewEmitter()
	gotIR, errs := e.Emit(ir)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}

	got := gotIR.HTTPRoutes[key].Spec.Rules[0].Filters
	if len(got) != 1 {
		t.Fatalf("expected 1 filter, got %d: %#v", len(got), got)
	}

	f := got[0]
	if f.Type != gatewayv1.HTTPRouteFilterURLRewrite {
		t.Fatalf("expected filter type %q, got %q", gatewayv1.HTTPRouteFilterURLRewrite, f.Type)
	}
	if f.URLRewrite == nil || f.URLRewrite.Path == nil {
		t.Fatalf("expected URLRewrite.Path to be set, got: %#v", f.URLRewrite)
	}
	if f.URLRewrite.Path.Type != gatewayv1.FullPathHTTPPathModifier {
		t.Fatalf("expected Path.Type %q, got %q", gatewayv1.FullPathHTTPPathModifier, f.URLRewrite.Path.Type)
	}
	if f.URLRewrite.Path.ReplaceFullPath == nil || *f.URLRewrite.Path.ReplaceFullPath != "/foo" {
		t.Fatalf("expected ReplaceFullPath /foo, got: %#v", f.URLRewrite.Path.ReplaceFullPath)
	}
}
