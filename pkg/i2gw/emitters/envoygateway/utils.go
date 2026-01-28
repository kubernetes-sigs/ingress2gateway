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
		if (first.BufferSize == nil) != (bs.BufferSize == nil) {
			return
		}
		if first.BufferSize != nil && !first.BufferSize.Equal(*bs.BufferSize) {
			return
		}
		if (first.MaxSize == nil) != (bs.MaxSize == nil) {
			return
		}
		if first.MaxSize != nil && !first.MaxSize.Equal(*bs.MaxSize) {
			return
		}
	}

	ctx.BodySizeByRuleIdx = map[int]*emitterir.BodySize{
		RouteRuleAllIndex: first,
	}
}
