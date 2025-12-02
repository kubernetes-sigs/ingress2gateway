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
	"time"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const nginxProxyConnectTimeout = "nginx.ingress.kubernetes.io/proxy-connect-timeout"

// proxyConnectTimeoutFeature parses proxy-connect-timeout and stores it in the IR Policy.
//
// Semantics:
//   - Annotation value is treated like nginx: either a bare number of seconds ("30")
//     or a Go-style duration ("30s", "2m", ...).
//   - We normalize it to metav1.Duration and attach it per-Ingress, then map to
//     specific (rule, backend) pairs via RuleBackendSources.
func proxyConnectTimeoutFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	// Per-Ingress parsed timeout.
	perIngress := map[types.NamespacedName]*metav1.Duration{}

	for i := range ingresses {
		ing := &ingresses[i]
		raw := ing.Annotations[nginxProxyConnectTimeout]
		if raw == "" {
			continue
		}

		// Try parsing as a Go duration first ("30s", "2m", etc.).
		d, err := time.ParseDuration(raw)
		if err != nil {
			// Fallback: assume bare seconds ("30").
			d, err = time.ParseDuration(raw + "s")
		}
		if err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(nginxProxyConnectTimeout),
				raw,
				"failed to parse proxy-connect-timeout",
			))
			continue
		}

		key := types.NamespacedName{
			Namespace: ing.Namespace,
			Name:      ing.Name,
		}
		perIngress[key] = &metav1.Duration{Duration: d}
	}

	if len(perIngress) == 0 {
		return errs
	}

	// Map per-Ingress timeout onto HTTPRoute policies using RuleBackendSources.
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

		if httpCtx.ProviderSpecificIR.IngressNginx == nil {
			httpCtx.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
				Policies: map[string]intermediate.Policy{},
			}
		}
		if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
			httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
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

				timeout := perIngress[ingKey]
				if timeout == nil {
					continue
				}

				p := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if p.ProxyConnectTimeout == nil {
					p.ProxyConnectTimeout = timeout
				}

				// Dedupe (rule, backend) pairs.
				p.AddRuleBackendSources([]intermediate.PolicyIndex{
					{
						Rule:    ruleIdx,
						Backend: backendIdx,
					},
				})

				httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = p
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}
