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

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
)

func MergeBodySizeIR(ctx *emitterir.HTTPRouteContext) {
	if len(ctx.BodySizeByRuleIdx) != len(ctx.Spec.Rules) {
		return
	}

	var first *emitterir.BodySize
	for _, bs := range ctx.BodySizeByRuleIdx {
		if first == nil {
			first = bs
			continue
		}
		if !reflect.DeepEqual(first, bs) {
			return
		}
	}

	ctx.BodySizeByRuleIdx = map[int]*emitterir.BodySize{
		RouteRuleAllIndex: first,
	}
}

func MergeCorsIR(ctx *emitterir.HTTPRouteContext) {
	if len(ctx.CorsPolicyByRuleIdx) != len(ctx.Spec.Rules) {
		return
	}

	var first *gatewayv1.HTTPCORSFilter
	for _, cp := range ctx.CorsPolicyByRuleIdx {
		if first == nil {
			first = cp
			continue
		}
		if !reflect.DeepEqual(first, cp) {
			return
		}
	}

	ctx.CorsPolicyByRuleIdx = map[int]*gatewayv1.HTTPCORSFilter{
		RouteRuleAllIndex: first,
	}
}
