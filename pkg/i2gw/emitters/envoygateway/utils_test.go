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

package envoygateway_emitter

import (
	"reflect"
	"testing"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestMergeBodySizeIR(t *testing.T) {
	maxSize10M := resource.MustParse("10M")
	maxSize20M := resource.MustParse("20M")
	bufferSize5M := resource.MustParse("5M")
	bufferSize8M := resource.MustParse("8M")

	tests := []struct {
		name         string
		numRules     int
		bodySizeMap  map[int]*emitterir.BodySize
		wantMerged   bool
		wantBodySize *emitterir.BodySize
	}{
		{
			name:     "all rules have same body size - should merge",
			numRules: 3,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {MaxSize: &maxSize10M, BufferSize: &bufferSize5M},
				1: {MaxSize: &maxSize10M, BufferSize: &bufferSize5M},
				2: {MaxSize: &maxSize10M, BufferSize: &bufferSize5M},
			},
			wantMerged:   true,
			wantBodySize: &emitterir.BodySize{MaxSize: &maxSize10M, BufferSize: &bufferSize5M},
		},
		{
			name:     "all rules have same max size only - should merge",
			numRules: 2,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {MaxSize: &maxSize10M},
				1: {MaxSize: &maxSize10M},
			},
			wantMerged:   true,
			wantBodySize: &emitterir.BodySize{MaxSize: &maxSize10M},
		},
		{
			name:     "all rules have same buffer size only - should merge",
			numRules: 2,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {BufferSize: &bufferSize5M},
				1: {BufferSize: &bufferSize5M},
			},
			wantMerged:   true,
			wantBodySize: &emitterir.BodySize{BufferSize: &bufferSize5M},
		},
		{
			name:     "different max size - should not merge",
			numRules: 2,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {MaxSize: &maxSize10M},
				1: {MaxSize: &maxSize20M},
			},
			wantMerged: false,
		},
		{
			name:     "different buffer size - should not merge",
			numRules: 2,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {BufferSize: &bufferSize5M},
				1: {BufferSize: &bufferSize8M},
			},
			wantMerged: false,
		},
		{
			name:     "one has buffer size, one doesn't - should not merge",
			numRules: 2,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {MaxSize: &maxSize10M, BufferSize: &bufferSize5M},
				1: {MaxSize: &maxSize10M},
			},
			wantMerged: false,
		},
		{
			name:     "one has max size, one doesn't - should not merge",
			numRules: 2,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {MaxSize: &maxSize10M},
				1: {BufferSize: &bufferSize5M},
			},
			wantMerged: false,
		},
		{
			name:     "body size map length doesn't match rules - should not merge",
			numRules: 3,
			bodySizeMap: map[int]*emitterir.BodySize{
				0: {MaxSize: &maxSize10M},
				1: {MaxSize: &maxSize10M},
			},
			wantMerged: false,
		},
		{
			name:        "empty body size map - should not merge",
			numRules:    2,
			bodySizeMap: map[int]*emitterir.BodySize{},
			wantMerged:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create HTTPRouteContext with specified number of rules
			ctx := &emitterir.HTTPRouteContext{
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-route",
						Namespace: "default",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: make([]gatewayv1.HTTPRouteRule, tt.numRules),
					},
				},
				BodySizeByRuleIdx: tt.bodySizeMap,
			}

			// Call MergeBodySizeIR
			MergeBodySizeIR(ctx)

			if tt.wantMerged {
				// Should have merged to RouteRuleAllIndex
				if len(ctx.BodySizeByRuleIdx) != 1 {
					t.Errorf("expected BodySizeByRuleIdx to have 1 entry, got %d", len(ctx.BodySizeByRuleIdx))
					return
				}

				merged, ok := ctx.BodySizeByRuleIdx[RouteRuleAllIndex]
				if !ok {
					t.Errorf("expected BodySizeByRuleIdx to have entry at RouteRuleAllIndex=%d", RouteRuleAllIndex)
					return
				}

				// Check MaxSize
				if tt.wantBodySize.MaxSize == nil {
					if merged.MaxSize != nil {
						t.Errorf("expected MaxSize to be nil, got %v", merged.MaxSize)
					}
				} else {
					if merged.MaxSize == nil {
						t.Errorf("expected MaxSize to be %v, got nil", tt.wantBodySize.MaxSize)
					} else if !merged.MaxSize.Equal(*tt.wantBodySize.MaxSize) {
						t.Errorf("expected MaxSize %v, got %v", tt.wantBodySize.MaxSize, merged.MaxSize)
					}
				}

				// Check BufferSize
				if tt.wantBodySize.BufferSize == nil {
					if merged.BufferSize != nil {
						t.Errorf("expected BufferSize to be nil, got %v", merged.BufferSize)
					}
				} else {
					if merged.BufferSize == nil {
						t.Errorf("expected BufferSize to be %v, got nil", tt.wantBodySize.BufferSize)
					} else if !merged.BufferSize.Equal(*tt.wantBodySize.BufferSize) {
						t.Errorf("expected BufferSize %v, got %v", tt.wantBodySize.BufferSize, merged.BufferSize)
					}
				}
			} else {
				// Should not have merged - BodySizeByRuleIdx should be unchanged
				if len(ctx.BodySizeByRuleIdx) != len(tt.bodySizeMap) {
					t.Errorf("expected BodySizeByRuleIdx length to remain %d, got %d", len(tt.bodySizeMap), len(ctx.BodySizeByRuleIdx))
				}
				if _, exists := ctx.BodySizeByRuleIdx[RouteRuleAllIndex]; exists {
					t.Errorf("expected no entry at RouteRuleAllIndex=%d, but found one", RouteRuleAllIndex)
				}
			}
		})
	}
}

func TestMergeCorsIR(t *testing.T) {
	corsPolicy1 := &gatewayv1.HTTPCORSFilter{
		AllowOrigins:     []gatewayv1.CORSOrigin{"https://example.com", "https://foo.com"},
		AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "POST"},
		AllowHeaders:     []gatewayv1.HTTPHeaderName{"Content-Type", "Authorization"},
		ExposeHeaders:    []gatewayv1.HTTPHeaderName{"X-Custom-Header"},
		AllowCredentials: ptr.To(true),
		MaxAge:           3600,
	}

	corsPolicySame := &gatewayv1.HTTPCORSFilter{
		AllowOrigins:     []gatewayv1.CORSOrigin{"https://example.com", "https://foo.com"},
		AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "POST"},
		AllowHeaders:     []gatewayv1.HTTPHeaderName{"Content-Type", "Authorization"},
		ExposeHeaders:    []gatewayv1.HTTPHeaderName{"X-Custom-Header"},
		AllowCredentials: ptr.To(true),
		MaxAge:           3600,
	}

	corsPolicyDifferentOrigins := &gatewayv1.HTTPCORSFilter{
		AllowOrigins:     []gatewayv1.CORSOrigin{"https://different.com"},
		AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "POST"},
		AllowHeaders:     []gatewayv1.HTTPHeaderName{"Content-Type", "Authorization"},
		ExposeHeaders:    []gatewayv1.HTTPHeaderName{"X-Custom-Header"},
		AllowCredentials: ptr.To(true),
		MaxAge:           3600,
	}

	corsPolicyDifferentMethods := &gatewayv1.HTTPCORSFilter{
		AllowOrigins:     []gatewayv1.CORSOrigin{"https://example.com", "https://foo.com"},
		AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "PUT"},
		AllowHeaders:     []gatewayv1.HTTPHeaderName{"Content-Type", "Authorization"},
		ExposeHeaders:    []gatewayv1.HTTPHeaderName{"X-Custom-Header"},
		AllowCredentials: ptr.To(true),
		MaxAge:           3600,
	}

	corsPolicyDifferentCredentials := &gatewayv1.HTTPCORSFilter{
		AllowOrigins:     []gatewayv1.CORSOrigin{"https://example.com", "https://foo.com"},
		AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "POST"},
		AllowHeaders:     []gatewayv1.HTTPHeaderName{"Content-Type", "Authorization"},
		ExposeHeaders:    []gatewayv1.HTTPHeaderName{"X-Custom-Header"},
		AllowCredentials: ptr.To(false),
		MaxAge:           3600,
	}

	corsPolicyDifferentMaxAge := &gatewayv1.HTTPCORSFilter{
		AllowOrigins:     []gatewayv1.CORSOrigin{"https://example.com", "https://foo.com"},
		AllowMethods:     []gatewayv1.HTTPMethodWithWildcard{"GET", "POST"},
		AllowHeaders:     []gatewayv1.HTTPHeaderName{"Content-Type", "Authorization"},
		ExposeHeaders:    []gatewayv1.HTTPHeaderName{"X-Custom-Header"},
		AllowCredentials: ptr.To(true),
		MaxAge:           7200,
	}

	tests := []struct {
		name           string
		numRules       int
		corsPolicyMap  map[int]*gatewayv1.HTTPCORSFilter
		wantMerged     bool
		wantCorsPolicy *gatewayv1.HTTPCORSFilter
	}{
		{
			name:     "all rules have same CORS policy - should merge",
			numRules: 3,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{
				0: corsPolicy1,
				1: corsPolicySame,
				2: corsPolicy1,
			},
			wantMerged:     true,
			wantCorsPolicy: corsPolicy1,
		},
		{
			name:     "two rules with same CORS policy - should merge",
			numRules: 2,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{
				0: corsPolicy1,
				1: corsPolicySame,
			},
			wantMerged:     true,
			wantCorsPolicy: corsPolicy1,
		},
		{
			name:     "different origins - should not merge",
			numRules: 2,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{
				0: corsPolicy1,
				1: corsPolicyDifferentOrigins,
			},
			wantMerged: false,
		},
		{
			name:     "different methods - should not merge",
			numRules: 2,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{
				0: corsPolicy1,
				1: corsPolicyDifferentMethods,
			},
			wantMerged: false,
		},
		{
			name:     "different credentials - should not merge",
			numRules: 2,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{
				0: corsPolicy1,
				1: corsPolicyDifferentCredentials,
			},
			wantMerged: false,
		},
		{
			name:     "different max age - should not merge",
			numRules: 2,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{
				0: corsPolicy1,
				1: corsPolicyDifferentMaxAge,
			},
			wantMerged: false,
		},
		{
			name:     "CORS policy map length doesn't match rules - should not merge",
			numRules: 3,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{
				0: corsPolicy1,
				1: corsPolicySame,
			},
			wantMerged: false,
		},
		{
			name:          "empty CORS policy map - should not merge",
			numRules:      2,
			corsPolicyMap: map[int]*gatewayv1.HTTPCORSFilter{},
			wantMerged:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create HTTPRouteContext with specified number of rules
			ctx := &emitterir.HTTPRouteContext{
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-route",
						Namespace: "default",
					},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: make([]gatewayv1.HTTPRouteRule, tt.numRules),
					},
				},
				CorsPolicyByRuleIdx: tt.corsPolicyMap,
			}

			MergeCorsIR(ctx)

			if tt.wantMerged {
				// Should have merged to RouteRuleAllIndex
				if len(ctx.CorsPolicyByRuleIdx) != 1 {
					t.Errorf("expected CorsPolicyByRuleIdx to have 1 entry, got %d", len(ctx.CorsPolicyByRuleIdx))
					return
				}

				merged, ok := ctx.CorsPolicyByRuleIdx[RouteRuleAllIndex]
				if !ok {
					t.Errorf("expected CorsPolicyByRuleIdx to have entry at RouteRuleAllIndex=%d", RouteRuleAllIndex)
					return
				}

				if merged == nil {
					t.Errorf("expected merged CORS policy to be non-nil")
					return
				}

				if !reflect.DeepEqual(merged, tt.wantCorsPolicy) {
					t.Errorf("merged CORS policy doesn't match expected.\ngot:  %+v\nwant: %+v", merged, tt.wantCorsPolicy)
				}
			} else {
				// Should not have merged - CorsPolicyByRuleIdx should be unchanged
				if len(ctx.CorsPolicyByRuleIdx) != len(tt.corsPolicyMap) {
					t.Errorf("expected CorsPolicyByRuleIdx length to remain %d, got %d", len(tt.corsPolicyMap), len(ctx.CorsPolicyByRuleIdx))
				}
				if _, exists := ctx.CorsPolicyByRuleIdx[RouteRuleAllIndex]; exists {
					t.Errorf("expected no entry at RouteRuleAllIndex=%d, but found one", RouteRuleAllIndex)
				}
			}
		})
	}
}
