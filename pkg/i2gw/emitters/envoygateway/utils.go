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

package envoygateway_emitter

import (
	"reflect"

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

func MergeIPRangeControlIR(ctx *emitterir.HTTPRouteContext) {
	if len(ctx.IPRangeControlByRuleIdx) != len(ctx.Spec.Rules) {
		return
	}

	var first *emitterir.IPRangeControl
	for _, iprc := range ctx.IPRangeControlByRuleIdx {
		if first == nil {
			first = iprc
			continue
		}
		if !reflect.DeepEqual(first, iprc) {
			return
		}
	}

	ctx.IPRangeControlByRuleIdx = map[int]*emitterir.IPRangeControl{
		RouteRuleAllIndex: first,
	}
}
