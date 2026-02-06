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
	"fmt"

	egapiv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

func (e *Emitter) EmitBuffer(ir emitterir.EmitterIR, gwResources *i2gw.GatewayResources) {
	for nn, ctx := range ir.HTTPRoutes {
		MergeBodySizeIR(&ctx)

		for idx, bs := range ctx.BodySizeByRuleIdx {
			// if BodySize is nil, it means that BodySize IR has already applied by common emitter.
			if bs == nil {
				continue
			}

			var sectionName *gwapiv1.SectionName
			if idx != RouteRuleAllIndex && idx < len(ctx.Spec.Rules) {
				sectionName = ctx.Spec.Rules[idx].Name
			}

			backendTrafficPolicy := e.getOrBuildBackendTrafficPolicy(ctx, sectionName, idx)

			// Prefer body max size if present, otherwise fall back to body buffer size.
			var bufferVal *resource.Quantity

			if bs.MaxSize != nil {
				bufferVal = bs.MaxSize
				if bs.BufferSize != nil {
					notify(
						notifications.WarningNotification,
						fmt.Sprintf("Body max size (%s) takes precedence; buffer size (%s) will be ignored", bs.MaxSize.String(), bs.BufferSize.String()),
						&ctx.HTTPRoute,
					)
				}
			} else if bs.BufferSize != nil {
				bufferVal = bs.BufferSize
			}
			backendTrafficPolicy.Spec.RequestBuffer = &egapiv1a1.RequestBuffer{
				Limit: *bufferVal,
			}

			ruleInfo := ""
			if sectionName != nil {
				ruleInfo = fmt.Sprintf(" rule %s", *sectionName)
			}
			notify(notifications.InfoNotification, fmt.Sprintf("applied Buffer feature for HTTPRoute%s", ruleInfo), &ctx.HTTPRoute)

			// mark Buffer IR as processed
			ctx.BodySizeByRuleIdx[idx] = nil
		}

		ir.HTTPRoutes[nn] = ctx
	}
}
