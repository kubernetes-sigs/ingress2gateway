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
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const clientBodyBufferSizeAnnotation = "nginx.ingress.kubernetes.io/client-body-buffer-size"

// bufferPolicyFeature parses the "nginx.ingress.kubernetes.io/client-body-buffer-size" annotation
// from Ingresses and records them as generic Policies in the ingress-nginx provider-specific IR.
func bufferPolicyFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errList field.ErrorList

	// Build per-Ingress policies based on the annotation.
	ingressPolicies := map[types.NamespacedName]*intermediate.Policy{}

	for i := range ingresses {
		ing := &ingresses[i]
		val := ing.Annotations[clientBodyBufferSizeAnnotation]
		if val == "" {
			continue
		}

		q, err := resource.ParseQuantity(val)
		if err != nil {
			errList = append(errList, field.Invalid(
				field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(clientBodyBufferSizeAnnotation),
				val,
				"failed to parse client-body-buffer-size",
			))
			continue
		}

		qCopy := q.DeepCopy()
		key := types.NamespacedName{
			Namespace: ing.Namespace,
			Name:      ing.Name,
		}
		ingressPolicies[key] = &intermediate.Policy{
			ClientBodyBufferSize: &qCopy,
		}
	}

	if len(ingressPolicies) == 0 {
		// No relevant annotations, nothing to do.
		return errList
	}

	// Use RuleBackendSources to map each ingress policy to specific
	// rule/backend indices on HTTPRoutes, populating provider-specific policies.
	ruleGroups := common.GetRuleGroups(ingresses)

	for _, rg := range ruleGroups {
		routeKey := types.NamespacedName{
			Namespace: rg.Namespace,
			Name:      common.RouteName(rg.Name, rg.Host),
		}

		httpRouteContext, ok := ir.HTTPRoutes[routeKey]
		if !ok {
			continue
		}

		for ruleIdx, backendSources := range httpRouteContext.RuleBackendSources {
			if ruleIdx >= len(httpRouteContext.HTTPRoute.Spec.Rules) {
				errList = append(errList, field.InternalError(
					field.NewPath("httproute", httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx),
					fmt.Errorf("rule index %d exceeds available rules", ruleIdx),
				))
				continue
			}

			for backendIdx, source := range backendSources {
				if source.Ingress == nil {
					continue
				}

				ingKey := types.NamespacedName{
					Namespace: source.Ingress.Namespace,
					Name:      source.Ingress.Name,
				}
				pol, ok := ingressPolicies[ingKey]
				if !ok {
					// This ingress has no buffer policy.
					continue
				}

				// Ensure provider-specific IR for ingress-nginx exists.
				if httpRouteContext.ProviderSpecificIR.IngressNginx == nil {
					httpRouteContext.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
						Policies: map[string]intermediate.Policy{},
					}
				}
				if httpRouteContext.ProviderSpecificIR.IngressNginx.Policies == nil {
					httpRouteContext.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
				}

				// Get or initialize the Policy for this ingress name.
				p := httpRouteContext.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]

				if p.ClientBodyBufferSize == nil && pol.ClientBodyBufferSize != nil {
					p.ClientBodyBufferSize = pol.ClientBodyBufferSize
				}

				// Dedupe (rule, backend) pairs.
				p = p.AddRuleBackendSources([]intermediate.PolicyIndex{
					{
						Rule:    ruleIdx,
						Backend: backendIdx,
					},
				})

				httpRouteContext.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = p
			}
		}

		// Write back updated context into the IR.
		ir.HTTPRoutes[routeKey] = httpRouteContext
	}

	return errList
}
