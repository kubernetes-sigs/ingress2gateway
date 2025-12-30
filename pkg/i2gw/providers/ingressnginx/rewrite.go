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

package ingressnginx

import (
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// applyRewriteTargetToEmitterIR is a temporary bridge until we decide how rewrite
// should be integrated into the generic feature parsing flow.
//
// It reads ingress-nginx rewrite annotations from ProviderIR sources and stores
// provider-neutral rewrite intent into EmitterIR, which will later be converted
// to Gateway API URLRewrite filters by the common emitter.
func applyRewriteTargetToEmitterIR(pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) field.ErrorList {
	for key, pRouteCtx := range pIR.HTTPRoutes {
		eRouteCtx, ok := eIR.HTTPRoutes[key]
		if !ok {
			continue
		}

		for ruleIdx := range eRouteCtx.Spec.Rules {
			if ruleIdx >= len(pRouteCtx.RuleBackendSources) {
				continue
			}
			ing := getNonCanaryIngress(pRouteCtx.RuleBackendSources[ruleIdx])
			if ing == nil {
				continue
			}

			rewriteTarget := ing.Annotations[RewriteTargetAnnotation]
			if rewriteTarget == "" {
				continue
			}

			if eRouteCtx.PathRewriteByRuleIdx == nil {
				eRouteCtx.PathRewriteByRuleIdx = make(map[int]*emitterir.PathRewrite)
			}

			pathRewriteIR := emitterir.PathRewrite{ReplaceFullPath: rewriteTarget}

			if val, ok := ing.Annotations[XForwardedPrefixAnnotation]; ok && val != "" {
				pathRewriteIR.Headers["X-Forwarded-Prefix"] = val
			}
			eRouteCtx.PathRewriteByRuleIdx[ruleIdx] = &pathRewriteIR
		}

		eIR.HTTPRoutes[key] = eRouteCtx
	}

	return nil
}
