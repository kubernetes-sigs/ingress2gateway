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

package agentgateway

import (
	"fmt"
	"strings"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	"github.com/agentgateway/agentgateway/controller/api/v1alpha1/shared"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyRewriteTargetPolicy projects ingress-nginx rewrite-target into AgentgatewayPolicy.traffic.transformation.
//
// Semantics:
//   - If rewrite-target is configured, set traffic.transformation.request.set to include:
//   - name: :path
//     value: <CEL expression>
//   - For non-regex paths, value is a CEL string literal for the rewrite target (ReplaceFullPath semantics).
//   - For use-regex=true, value is a CEL expression:
//     regexReplace(request.path, "<pattern>", "<substitution>")
//
// Notes:
//   - AgentgatewayPolicy is attached at the HTTPRoute scope. The emitter "full coverage" check will reject subset
//     coverage and avoid producing partially-applied rewrites.
//   - If no single regex match pattern can be derived from the HTTPRoute rules, we fall back to "^(.*)".
func applyRewriteTargetPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	httpRouteCtx *emitterir.HTTPRouteContext,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.RewriteTarget == nil {
		return false
	}
	target := strings.TrimSpace(*pol.RewriteTarget)
	if target == "" {
		return false
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Traffic == nil {
		agp.Spec.Traffic = &agentgatewayv1alpha1.Traffic{}
	}

	// Ensure nested structs exist (Traffic.Transformation -> Transformation.Request -> Transform.Set).
	if agp.Spec.Traffic.Transformation == nil {
		agp.Spec.Traffic.Transformation = &agentgatewayv1alpha1.Transformation{}
	}
	if agp.Spec.Traffic.Transformation.Request == nil {
		agp.Spec.Traffic.Transformation.Request = &agentgatewayv1alpha1.Transform{}
	}

	// Default: literal replace (CEL string literal).
	// NOTE: shared.CELExpression is the API type for CEL snippets; a literal string must include quotes.
	expr := shared.CELExpression(fmt.Sprintf("%q", target))

	// Regex: best-effort regexReplace over request.path.
	if pol.UseRegexPaths != nil && *pol.UseRegexPaths {
		pattern := deriveRewriteTargetRegexPattern(httpRouteCtx)
		expr = shared.CELExpression(
			fmt.Sprintf("regexReplace(request.path, %q, %q)", pattern, target),
		)
	}

	// Agentgateway expresses request/response mutations via Traffic.Transformation, which operates on HTTP headers.
	// The HeaderName type explicitly supports HTTP pseudo-headers (including ":path").
	upsertHeaderTransformation(
		&agp.Spec.Traffic.Transformation.Request.Set,
		agentgatewayv1alpha1.HeaderName(":path"),
		expr,
	)

	ap[ingressName] = agp
	return true
}

// upsertHeaderTransformation sets (or appends) a HeaderTransformation with the given name.
// This avoids emitting duplicate entries when multiple features touch the same header.
func upsertHeaderTransformation(
	list *[]agentgatewayv1alpha1.HeaderTransformation,
	name agentgatewayv1alpha1.HeaderName,
	value shared.CELExpression,
) {
	if list == nil {
		return
	}

	for i := range *list {
		if (*list)[i].Name == name {
			(*list)[i].Value = value
			return
		}
	}

	*list = append(*list, agentgatewayv1alpha1.HeaderTransformation{
		Name:  name,
		Value: value,
	})
}

// deriveRewriteTargetRegexPattern attempts to derive a single regex pattern from the HTTPRoute rules.
//
// If the route has:
//   - exactly one distinct RegularExpression path value across all rules -> return it
//   - zero or multiple distinct regex values                             -> fall back to "^(.*)"
func deriveRewriteTargetRegexPattern(httpRouteCtx *emitterir.HTTPRouteContext) string {
	if httpRouteCtx == nil {
		return "^(.*)"
	}

	patterns := map[string]struct{}{}

	for _, rule := range httpRouteCtx.Spec.Rules {
		for i := range rule.Matches {
			m := rule.Matches[i]
			if m.Path == nil || m.Path.Type == nil || m.Path.Value == nil {
				continue
			}
			if *m.Path.Type != gatewayv1.PathMatchRegularExpression {
				continue
			}
			if v := *m.Path.Value; v != "" {
				patterns[v] = struct{}{}
			}
		}
	}

	if len(patterns) == 1 {
		for p := range patterns {
			return p
		}
	}

	return "^(.*)"
}
