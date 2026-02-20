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
	"testing"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestMergeIPRangeControlIR(t *testing.T) {
	allowList1 := []string{"192.168.1.0/24", "10.0.0.0/8"}
	allowList2 := []string{"172.16.0.0/12"}
	denyList1 := []string{"203.0.113.0/24"}
	denyList2 := []string{"198.51.100.0/24"}

	tests := []struct {
		name               string
		numRules           int
		ipRangeControlMap  map[int]*emitterir.IPRangeControl
		wantMerged         bool
		wantIPRangeControl *emitterir.IPRangeControl
	}{
		{
			name:     "all rules have same IP range control - should merge",
			numRules: 3,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {AllowList: allowList1, DenyList: denyList1},
				1: {AllowList: allowList1, DenyList: denyList1},
				2: {AllowList: allowList1, DenyList: denyList1},
			},
			wantMerged:         true,
			wantIPRangeControl: &emitterir.IPRangeControl{AllowList: allowList1, DenyList: denyList1},
		},
		{
			name:     "all rules have same allow list only - should merge",
			numRules: 2,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {AllowList: allowList1},
				1: {AllowList: allowList1},
			},
			wantMerged:         true,
			wantIPRangeControl: &emitterir.IPRangeControl{AllowList: allowList1},
		},
		{
			name:     "all rules have same deny list only - should merge",
			numRules: 2,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {DenyList: denyList1},
				1: {DenyList: denyList1},
			},
			wantMerged:         true,
			wantIPRangeControl: &emitterir.IPRangeControl{DenyList: denyList1},
		},
		{
			name:     "different allow list - should not merge",
			numRules: 2,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {AllowList: allowList1},
				1: {AllowList: allowList2},
			},
			wantMerged: false,
		},
		{
			name:     "different deny list - should not merge",
			numRules: 2,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {DenyList: denyList1},
				1: {DenyList: denyList2},
			},
			wantMerged: false,
		},
		{
			name:     "one has deny list, one doesn't - should not merge",
			numRules: 2,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {AllowList: allowList1, DenyList: denyList1},
				1: {AllowList: allowList1},
			},
			wantMerged: false,
		},
		{
			name:     "one has allow list, one doesn't - should not merge",
			numRules: 2,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {AllowList: allowList1},
				1: {DenyList: denyList1},
			},
			wantMerged: false,
		},
		{
			name:     "IP range control map length doesn't match rules - should not merge",
			numRules: 3,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{
				0: {AllowList: allowList1},
				1: {AllowList: allowList1},
			},
			wantMerged: false,
		},
		{
			name:              "empty IP range control map - should not merge",
			numRules:          2,
			ipRangeControlMap: map[int]*emitterir.IPRangeControl{},
			wantMerged:        false,
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
				IPRangeControlByRuleIdx: tt.ipRangeControlMap,
			}

			// Call MergeIPRangeControlIR
			MergeIPRangeControlIR(ctx)

			if tt.wantMerged {
				// Should have merged to RouteRuleAllIndex
				if len(ctx.IPRangeControlByRuleIdx) != 1 {
					t.Errorf("expected IPRangeControlByRuleIdx to have 1 entry, got %d", len(ctx.IPRangeControlByRuleIdx))
					return
				}

				merged, ok := ctx.IPRangeControlByRuleIdx[RouteRuleAllIndex]
				if !ok {
					t.Errorf("expected IPRangeControlByRuleIdx to have entry at RouteRuleAllIndex=%d", RouteRuleAllIndex)
					return
				}

				// Check AllowList
				if len(tt.wantIPRangeControl.AllowList) != len(merged.AllowList) {
					t.Errorf("expected AllowList length %d, got %d", len(tt.wantIPRangeControl.AllowList), len(merged.AllowList))
				} else {
					for i, cidr := range tt.wantIPRangeControl.AllowList {
						if merged.AllowList[i] != cidr {
							t.Errorf("expected AllowList[%d] = %s, got %s", i, cidr, merged.AllowList[i])
						}
					}
				}

				// Check DenyList
				if len(tt.wantIPRangeControl.DenyList) != len(merged.DenyList) {
					t.Errorf("expected DenyList length %d, got %d", len(tt.wantIPRangeControl.DenyList), len(merged.DenyList))
				} else {
					for i, cidr := range tt.wantIPRangeControl.DenyList {
						if merged.DenyList[i] != cidr {
							t.Errorf("expected DenyList[%d] = %s, got %s", i, cidr, merged.DenyList[i])
						}
					}
				}
			} else {
				// Should not have merged - IPRangeControlByRuleIdx should be unchanged
				if len(ctx.IPRangeControlByRuleIdx) != len(tt.ipRangeControlMap) {
					t.Errorf("expected IPRangeControlByRuleIdx length to remain %d, got %d", len(tt.ipRangeControlMap), len(ctx.IPRangeControlByRuleIdx))
				}
				if _, exists := ctx.IPRangeControlByRuleIdx[RouteRuleAllIndex]; exists {
					t.Errorf("expected no entry at RouteRuleAllIndex=%d, but found one", RouteRuleAllIndex)
				}
			}
		})
	}
}
