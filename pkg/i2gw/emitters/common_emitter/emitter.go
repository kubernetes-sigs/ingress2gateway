/*
Copyright 2025 The Kubernetes Authors.

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
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Emitter struct{}

func NewEmitter() *Emitter {
	return &Emitter{}
}

// Emit processes the IR to apply common logic (like deduplication) and returns the modified IR.
// This ALWAYS runs after providers and before provider-specific emitters.
// TODO: Implement common logic such as filtering by maturity status and/or individual features.
func (e *Emitter) Emit(ir emitterir.EmitterIR) (emitterir.EmitterIR, field.ErrorList) {
	var errs field.ErrorList

	for key, httpRouteContext := range ir.HTTPRoutes {
		errs = append(errs, applyHTTPRouteRequestTimeouts(&httpRouteContext)...)
		ir.HTTPRoutes[key] = httpRouteContext
	}

	if len(errs) > 0 {
		return ir, errs
	}
	return ir, nil
}

func applyHTTPRouteRequestTimeouts(httpRouteContext *emitterir.HTTPRouteContext) field.ErrorList {
	if httpRouteContext.RequestTimeouts == nil {
		return nil
	}

	var errs field.ErrorList
	for ruleIdx, d := range httpRouteContext.RequestTimeouts {
		if d == nil {
			continue
		}
		if ruleIdx < 0 || ruleIdx >= len(httpRouteContext.Spec.Rules) {
			errs = append(errs, field.Invalid(
				field.NewPath("httpRoute", "spec", "rules").Index(ruleIdx),
				ruleIdx,
				"rule index out of range",
			))
			continue
		}

		rule := &httpRouteContext.Spec.Rules[ruleIdx]
		if rule.Timeouts == nil {
			rule.Timeouts = &gatewayv1.HTTPRouteTimeouts{}
		}
		rule.Timeouts.Request = d
	}

	httpRouteContext.RequestTimeouts = nil
	return errs
}
