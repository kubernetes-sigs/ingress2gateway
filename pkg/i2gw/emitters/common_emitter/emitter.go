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

func applyPathRewrites(ir *emitterir.EmitterIR) {
	for key, routeCtx := range ir.HTTPRoutes {
		for ruleIdx, rewrite := range routeCtx.PathRewriteByRuleIdx {
			if rewrite == nil || rewrite.Regex {
				continue
			}
			fullPath := rewrite.ReplaceFullPath
			routeCtx.Spec.Rules[ruleIdx].Filters = append(routeCtx.Spec.Rules[ruleIdx].Filters, gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterURLRewrite,
				URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
					Path: &gatewayv1.HTTPPathModifier{
						Type:            gatewayv1.FullPathHTTPPathModifier,
						ReplaceFullPath: &fullPath,
					},
				},
			})
			if len(rewrite.Headers) > 0 {
				headerModifier := &gatewayv1.HTTPHeaderFilter{}
				for headerName, headerValue := range rewrite.Headers {
					headerModifier.Set = append(headerModifier.Set, gatewayv1.HTTPHeader{Name: gatewayv1.HTTPHeaderName(headerName), Value: headerValue})
				}
				routeCtx.Spec.Rules[ruleIdx].Filters = append(routeCtx.Spec.Rules[ruleIdx].Filters, gatewayv1.HTTPRouteFilter{
					Type:                  gatewayv1.HTTPRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: headerModifier,
				})
			}
			routeCtx.PathRewriteByRuleIdx[ruleIdx] = nil
		}

		ir.HTTPRoutes[key] = routeCtx
	}
}

// Emit processes the IR to apply common logic (like deduplication) and returns the modified IR.
// This ALWAYS runs after providers and before provider-specific emitters.
// TODO: Implement common logic such as filtering by maturity status and/or individual features.
func (e *Emitter) Emit(ir emitterir.EmitterIR) (emitterir.EmitterIR, field.ErrorList) {
	var errs field.ErrorList

	errs = append(errs, applyHTTPRouteRequestTimeouts(&ir)...)
	applyPathRewrites(&ir)
	return ir, nil
}

func applyHTTPRouteRequestTimeouts(ir *emitterir.EmitterIR) field.ErrorList {
	var errs field.ErrorList
	for i, httpRouteContext := range ir.HTTPRoutes {
		if httpRouteContext.RequestTimeouts == nil {
			return nil
		}

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
		ir.HTTPRoutes[i] = httpRouteContext
	}
	return errs
}
