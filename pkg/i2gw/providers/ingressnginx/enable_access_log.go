/*
Copyright 2024 The Kubernetes Authors.

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
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// this is actually enabled by default
const enableAccessLogAnnotation = "nginx.ingress.kubernetes.io/enable-access-log"

// enableAccessLogFeature extracts the "enable-access-log" annotation and
// projects it into the provider-specific IR similarly to other annotation features.
func enableAccessLogFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {

	var errs field.ErrorList
	ingressPolicies := map[types.NamespacedName]*intermediate.Policy{}

	for i := range ingresses {
		ing := &ingresses[i]
		raw := strings.TrimSpace(ing.Annotations[enableAccessLogAnnotation])
		if raw == "" {
			continue
		}

		// Parse boolean value - only "true" (case-sensitive) enables access log
		enableAccessLog := raw == "true"

		key := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
		pol := ingressPolicies[key]
		if pol == nil {
			pol = &intermediate.Policy{}
			ingressPolicies[key] = pol
		}

		pol.EnableAccessLog = &enableAccessLog
	}

	if len(ingressPolicies) == 0 {
		return errs
	}

	// Map policies to HTTPRoutes (same pattern as other features)
	ruleGroups := common.GetRuleGroups(ingresses)

	for _, rg := range ruleGroups {
		routeKey := types.NamespacedName{
			Namespace: rg.Namespace,
			Name:      common.RouteName(rg.Name, rg.Host),
		}

		httpCtx, ok := ir.HTTPRoutes[routeKey]
		if !ok {
			continue
		}

		for ruleIdx, backendSources := range httpCtx.RuleBackendSources {
			for backendIdx, src := range backendSources {
				if src.Ingress == nil {
					continue
				}

				ingKey := types.NamespacedName{
					Namespace: src.Ingress.Namespace,
					Name:      src.Ingress.Name,
				}

				pol := ingressPolicies[ingKey]
				if pol == nil || pol.EnableAccessLog == nil {
					continue
				}

				// Ensure provider-specific IR exists
				if httpCtx.ProviderSpecificIR.IngressNginx == nil {
					httpCtx.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
						Policies: map[string]intermediate.Policy{},
					}
				} else if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
					httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
				}

				existing := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if existing.EnableAccessLog == nil {
					existing.EnableAccessLog = pol.EnableAccessLog
				}

				// Dedupe (rule, backend) pairs.
				existing = existing.AddRuleBackendSources([]intermediate.PolicyIndex{
					{Rule: ruleIdx, Backend: backendIdx},
				})

				httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = existing
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}
