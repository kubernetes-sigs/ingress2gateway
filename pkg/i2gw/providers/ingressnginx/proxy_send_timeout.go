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
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const proxySendTimeoutAnnotation = "nginx.ingress.kubernetes.io/proxy-send-timeout"

// proxySendTimeoutFeature parses proxy-send-timeout and stores it on the IR Policy.
// TODO [danehans]: Set the `grpc_send_timeout` when gRPC support is added.
func proxySendTimeoutFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	perIngress := map[types.NamespacedName]*metav1.Duration{}

	for i := range ingresses {
		ing := &ingresses[i]
		raw := ing.Annotations[proxySendTimeoutAnnotation]
		if raw == "" {
			continue
		}

		// nginx uses seconds (e.g. "30", "60s"). time.ParseDuration handles both if we normalize.
		d, err := time.ParseDuration(raw)
		if err != nil {
			// try seconds fallback (e.g. "30")
			d, err = time.ParseDuration(raw + "s")
		}
		if err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(proxySendTimeoutAnnotation),
				raw,
				"failed to parse proxy-send-timeout",
			))
			continue
		}

		perIngress[types.NamespacedName{
			Namespace: ing.Namespace,
			Name:      ing.Name,
		}] = &metav1.Duration{Duration: d}
	}

	if len(perIngress) == 0 {
		return errs
	}

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
			httpCtx.ProviderSpecificIR.IngressNginx =
				&intermediate.IngressNginxHTTPRouteIR{Policies: map[string]intermediate.Policy{}}
		}

		for ruleIdx, backendSources := range httpCtx.RuleBackendSources {
			for backendIdx, src := range backendSources {
				if src.Ingress == nil {
					continue
				}

				key := types.NamespacedName{
					Namespace: src.Ingress.Namespace,
					Name:      src.Ingress.Name,
				}

				dur := perIngress[key]
				if dur == nil {
					continue
				}

				p := httpCtx.ProviderSpecificIR.IngressNginx.Policies[key.Name]
				if p.ProxySendTimeout == nil {
					p.ProxySendTimeout = dur
				}

				p.RuleBackendSources = append(
					p.RuleBackendSources,
					intermediate.PolicyIndex{Rule: ruleIdx, Backend: backendIdx},
				)

				httpCtx.ProviderSpecificIR.IngressNginx.Policies[key.Name] = p
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}
