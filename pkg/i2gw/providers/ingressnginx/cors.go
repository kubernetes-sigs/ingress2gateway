/*
Copyright 2023 The Kubernetes Authors.

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
	"strings"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	nginxEnableCORSAnnotation      = "nginx.ingress.kubernetes.io/enable-cors"
	nginxCORSAllowOriginAnnotation = "nginx.ingress.kubernetes.io/cors-allow-origin"
)

// corsPolicyFeature is a FeatureParser that projects enable-cors/cors-allow-origin
// into the ingress-nginx ProviderSpecificIR.
//
// Semantics:
//   - If enable-cors is "true" (case-sensitive),
//   - and cors-allow-origin is non-empty (comma-separated list),
//   - we record a CorsPolicy for that Ingress and mark which
//     (rule,backend) indices in the merged HTTPRoute it contributed.
func corsPolicyFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	// Build per-Ingress policy from the CORS annotations.
	ing2pol := make(map[string]intermediate.Policy, len(ingresses))

	for _, ing := range ingresses {
		if ing.Annotations == nil {
			continue
		}

		enableRaw := strings.TrimSpace(ing.Annotations[nginxEnableCORSAnnotation])
		if enableRaw == "" || enableRaw != "true" {
			continue
		}

		allowRaw := strings.TrimSpace(ing.Annotations[nginxCORSAllowOriginAnnotation])
		if allowRaw == "" {
			// Common nginx behavior is to default to "*".
			allowRaw = "*"
		}

		var origins []string
		for _, part := range strings.Split(allowRaw, ",") {
			v := strings.TrimSpace(part)
			if v != "" {
				origins = append(origins, v)
			}
		}
		if len(origins) == 0 {
			continue
		}

		pol := ing2pol[ing.Name]
		if pol.Cors == nil {
			pol.Cors = &intermediate.CorsPolicy{}
		}
		pol.Cors.Enable = true
		pol.Cors.AllowOrigin = append(pol.Cors.AllowOrigin, origins...)
		ing2pol[ing.Name] = pol
	}

	if len(ing2pol) == 0 {
		return errs
	}

	// Map policies onto HTTPRoute rules/backends using BackendSource.
	for key, httpCtx := range ir.HTTPRoutes {
		// Group BackendSources by source Ingress name.
		srcByIngress := map[string][]intermediate.PolicyIndex{}

		for ruleIdx, perRule := range httpCtx.RuleBackendSources {
			for backendIdx, src := range perRule {
				if src.Ingress == nil {
					continue
				}
				ingressName := src.Ingress.Name
				srcByIngress[ingressName] = append(
					srcByIngress[ingressName],
					intermediate.PolicyIndex{Rule: ruleIdx, Backend: backendIdx},
				)
			}
		}

		if httpCtx.ProviderSpecificIR.IngressNginx == nil {
			httpCtx.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
				Policies: map[string]intermediate.Policy{},
			}
		} else if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
			httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
		}

		for ingressName, idxs := range srcByIngress {
			pol, ok := ing2pol[ingressName]
			if !ok || pol.Cors == nil {
				continue
			}

			existing := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingressName]

			// Merge CORS into existing policy for this Ingress (if any).
			if existing.Cors == nil {
				existing.Cors = pol.Cors
			} else {
				existing.Cors.Enable = existing.Cors.Enable || pol.Cors.Enable
				existing.Cors.AllowOrigin = append(existing.Cors.AllowOrigin, pol.Cors.AllowOrigin...)
			}

			// Dedupe (rule, backend) pairs.
			existing = existing.AddRuleBackendSources(idxs)

			httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingressName] = existing
		}

		// Write back mutated HTTPRouteContext into IR.
		ir.HTTPRoutes[key] = httpCtx
	}

	return errs
}
