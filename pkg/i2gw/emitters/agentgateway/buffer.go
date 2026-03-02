/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agentgateway

import (
	"fmt"
	"math"

	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// EmitBuffer converts BodySize intent from the emitter IR into AgentgatewayPolicy frontend buffer limits.
func (e *Emitter) EmitBuffer(ir emitterir.EmitterIR) {
	for _, ctx := range ir.HTTPRoutes {
		for idx, bs := range ctx.BodySizeByRuleIdx {
			if bs == nil {
				continue
			}

			sectionName := e.getSectionName(ctx, idx)
			policy := e.getOrBuildPolicy(ctx, sectionName, idx)
			bufferQty := e.selectBufferValue(bs, &ctx.HTTPRoute)
			if bufferQty == nil {
				continue
			}

			bufferSize := e.quantityToInt32(bufferQty, &ctx.HTTPRoute)
			if bufferSize == nil {
				continue
			}

			if policy.Spec.Frontend == nil {
				policy.Spec.Frontend = &agentgatewayv1alpha1.Frontend{}
			}
			if policy.Spec.Frontend.HTTP == nil {
				policy.Spec.Frontend.HTTP = &agentgatewayv1alpha1.FrontendHTTP{}
			}
			policy.Spec.Frontend.HTTP.MaxBufferSize = bufferSize

			notify(notifications.InfoNotification, fmt.Sprintf("applied buffer policy for HTTPRoute%s", e.formatRuleInfo(sectionName)), &ctx.HTTPRoute)
		}
	}
}

func (e *Emitter) getSectionName(ctx emitterir.HTTPRouteContext, idx int) *gatewayv1.SectionName {
	if idx != RouteRuleAllIndex && idx < len(ctx.Spec.Rules) {
		return ctx.Spec.Rules[idx].Name
	}
	return nil
}

func (e *Emitter) selectBufferValue(bs *emitterir.BodySize, httpRoute *gatewayv1.HTTPRoute) *resource.Quantity {
	if bs.MaxSize != nil {
		if bs.BufferSize != nil {
			notify(
				notifications.WarningNotification,
				fmt.Sprintf("body max size (%s) takes precedence; buffer size (%s) will be ignored", bs.MaxSize.String(), bs.BufferSize.String()),
				httpRoute,
			)
		}
		return bs.MaxSize
	}
	return bs.BufferSize
}

func (e *Emitter) formatRuleInfo(sectionName *gatewayv1.SectionName) string {
	if sectionName != nil {
		return fmt.Sprintf(" rule %s", *sectionName)
	}
	return ""
}

func (e *Emitter) quantityToInt32(q *resource.Quantity, httpRoute *gatewayv1.HTTPRoute) *int32 {
	if q == nil {
		return nil
	}
	val := q.Value()
	if val <= 0 {
		return nil
	}
	if val > math.MaxInt32 {
		notify(notifications.WarningNotification, fmt.Sprintf("buffer value %s exceeds max (%d); using %d", q.String(), math.MaxInt32, math.MaxInt32), httpRoute)
		val = math.MaxInt32
	}
	v := int32(val)
	return &v
}

func notify(mType notifications.MessageType, message string, callingObject ...client.Object) {
	newNotification := notifications.NewNotification(mType, message, callingObject...)
	notifications.NotificationAggr.DispatchNotification(newNotification, emitterName)
}
