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

package kgateway

import (
	"fmt"

	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/kgateway"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	envoygateway_emitter "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/envoygateway"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/api/resource"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// EmitBuffer processes BodySizeByRuleIdx from emitterIR and creates TrafficPolicies with buffer configuration.
func (e *Emitter) EmitBuffer(ir emitterir.EmitterIR) {
	for _, ctx := range ir.HTTPRoutes {
		envoygateway_emitter.MergeBodySizeIR(&ctx)

		for idx, bs := range ctx.BodySizeByRuleIdx {
			if bs == nil {
				continue
			}

			sectionName := e.getSectionName(ctx, idx)
			trafficPolicy := e.getOrBuildTrafficPolicy(ctx, sectionName, idx)

			bufferVal := e.selectBufferValue(bs, &ctx.HTTPRoute)
			if bufferVal == nil {
				continue
			}

			trafficPolicy.Spec.Buffer = &kgateway.Buffer{
				MaxRequestSize: bufferVal,
			}

			ruleInfo := e.formatRuleInfo(sectionName)
			e.notify(notifications.InfoNotification, fmt.Sprintf("applied Buffer feature for HTTPRoute%s", ruleInfo), &ctx.HTTPRoute)
		}
	}
}

// getSectionName returns the section name for the given rule index, or nil if it applies to all rules.
func (e *Emitter) getSectionName(ctx emitterir.HTTPRouteContext, idx int) *gatewayv1.SectionName {
	if idx != RouteRuleAllIndex && idx < len(ctx.Spec.Rules) {
		return ctx.Spec.Rules[idx].Name
	}
	return nil
}

// selectBufferValue selects the buffer value, preferring MaxSize over BufferSize, and emits a warning if both are present.
func (e *Emitter) selectBufferValue(bs *emitterir.BodySize, httpRoute *gatewayv1.HTTPRoute) *resource.Quantity {
	if bs.MaxSize != nil {
		if bs.BufferSize != nil {
			e.notify(
				notifications.WarningNotification,
				fmt.Sprintf("Body max size (%s) takes precedence; buffer size (%s) will be ignored", bs.MaxSize.String(), bs.BufferSize.String()),
				httpRoute,
			)
		}
		return bs.MaxSize
	}
	return bs.BufferSize
}

// formatRuleInfo formats the rule information for notification messages.
func (e *Emitter) formatRuleInfo(sectionName *gatewayv1.SectionName) string {
	if sectionName != nil {
		return fmt.Sprintf(" rule %s", *sectionName)
	}
	return ""
}
