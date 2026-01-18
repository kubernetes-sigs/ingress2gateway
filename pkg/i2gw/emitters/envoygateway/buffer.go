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
	egapiv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
)

func (e *Emitter) EmitBuffer(ir emitterir.EmitterIR, gwResources *i2gw.GatewayResources) {
	for _, ctx := range ir.HTTPRoutes {
		MergeBodySizeIR(&ctx, ctx.BodySizeByRuleIdx)

		for idx, bs := range ctx.BodySizeByRuleIdx {
			var sectionName *gwapiv1.SectionName
			if idx != RouteRuleAllIndex && idx < len(ctx.Spec.Rules) {
				sectionName = ctx.Spec.Rules[idx].Name
			}

			backendTrafficPolicy := e.getOrBuildBackendTrafficPolicy(ctx, sectionName, idx)

			// Prefer body max size if present, otherwise fall back to body buffer size.
			// TODO: add notification for which value from annotation is used.
			bufferVal := bs.MaxSize
			if bs.BufferSize != nil {
				bufferVal = bs.BufferSize
			}
			backendTrafficPolicy.Spec.RequestBuffer = &egapiv1a1.RequestBuffer{
				Limit: *bufferVal,
			}

			// TODO: add notification for successful buffer emission.
		}
	}
}
