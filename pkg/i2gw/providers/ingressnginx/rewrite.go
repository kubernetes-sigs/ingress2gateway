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
	"fmt"
	"strings"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// applyRewriteTargetToEmitterIR is a temporary bridge until we decide how rewrite
// should be integrated into the generic feature parsing flow.
//
// It reads ingress-nginx rewrite annotations from ProviderIR sources and stores
// provider-neutral rewrite intent into EmitterIR, which will later be converted
// to Gateway API URLRewrite filters by the common emitter.
func applyRewriteTargetToEmitterIR(ingresses []networkingv1.Ingress,
	pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) {
	hostsWithRegex := regexHosts(ingresses)

	for key, pRouteCtx := range pIR.HTTPRoutes {
		hasRegex := false
		for _, host := range pRouteCtx.Spec.Hostnames {
			if _, val := hostsWithRegex[string(host)]; val {
				hasRegex = true
				break
			}
		}

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

			pathRewriteIR := emitterir.PathRewrite{ReplaceFullPath: rewriteTarget, Headers: make(map[string]string)}

			if val, ok := ing.Annotations[XForwardedPrefixAnnotation]; ok && val != "" {
				pathRewriteIR.Headers["X-Forwarded-Prefix"] = val
			}

			if hasRegex && strings.Contains(rewriteTarget, "\\$") {
				pathRewriteIR.RegexCaptureGroupReferences = true
			}

			source := fmt.Sprintf("rewrite-target from Ingress %s/%s", ing.Namespace, ing.Name)
			paths := []*field.Path{field.NewPath("metadata", "annotations", fmt.Sprintf("%q", RewriteTargetAnnotation))}
			if hasRegex && strings.Contains(rewriteTarget, "\\$") {
				// Otherwise, rewrites without capture group references work.
				pathRewriteIR.RegexCaptureGroupReferences = true

				pathRewriteIR.Metadata = emitterir.NewExtensionFeatureMetadata(
					source,
					paths,
					"Path rewrites with capture group references are not supported",
				)
			} else {
				pathRewriteIR.Metadata = emitterir.NewExtensionFeatureMetadata(
					source,
					paths,
					"Could not apply rewrite-target annotation",
				)
			}

			eRouteCtx.PathRewriteByRuleIdx[ruleIdx] = &pathRewriteIR
		}

		eIR.HTTPRoutes[key] = eRouteCtx
	}
}
