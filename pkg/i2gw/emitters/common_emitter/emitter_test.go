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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errList := applyHTTPRouteRequestTimeouts(&tc.ctx)
			if tc.ctx.RequestTimeouts != nil {
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

			got := tc.ctx.Spec.Rules[0].Timeouts
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
