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
	"fmt"
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	canaryAnnotation            = "nginx.ingress.kubernetes.io/canary"
	canaryWeightAnnotation      = "nginx.ingress.kubernetes.io/canary-weight"
	canaryWeightTotalAnnotation = "nginx.ingress.kubernetes.io/canary-weight-total"
)

// canaryConfig holds the parsed canary configuration from a single Ingress
type canaryConfig struct {
	weight      int32
	weightTotal int32
}

// parseCanaryConfig extracts canary weight configuration from an Ingress
func parseCanaryConfig(ingress *networkingv1.Ingress) (canaryConfig, error) {
	config := canaryConfig{
		weight:      0,
		weightTotal: 100, // default
	}

	if weight := ingress.Annotations[canaryWeightAnnotation]; weight != "" {
		w, err := strconv.Atoi(weight)
		if err != nil {
			return config, fmt.Errorf("invalid canary-weight annotation %q: %w", weight, err)
		}
		if w < 0 {
			return config, fmt.Errorf("canary-weight must be non-negative, got %d", w)
		}
		config.weight = int32(w)
	}

	if total := ingress.Annotations[canaryWeightTotalAnnotation]; total != "" {
		wt, err := strconv.Atoi(total)
		if err != nil {
			return config, fmt.Errorf("invalid canary-weight-total annotation %q: %w", total, err)
		}
		if wt <= 0 {
			return config, fmt.Errorf("canary-weight-total must be positive, got %d", wt)
		}
		config.weightTotal = int32(wt)
	}

	if config.weight > config.weightTotal {
		return config, fmt.Errorf("canary-weight (%d) exceeds canary-weight-total (%d)", config.weight, config.weightTotal)
	}

	return config, nil
}

func canaryFeature(ingresses []networkingv1.Ingress, servicePorts map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errList field.ErrorList

	for _, rg := range ruleGroups {
		key := types.NamespacedName{Namespace: rg.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
		httpRouteContext, ok := ir.HTTPRoutes[key]
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

			// There must be a non canary backend and at most one canary backend
			// This is done in place.
			var canaryBackend *gatewayv1.HTTPBackendRef
			var nonCanaryBackend *gatewayv1.HTTPBackendRef
			var canaryConfig canaryConfig
			var canarySourceIngress *networkingv1.Ingress

			// Find the canary and non-canary backends
			for backendIdx, source := range backendSources {
				if source.Ingress == nil {
					continue
				}

				if backendIdx >= len(httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs) {
					errList = append(errList, field.InternalError(
						field.NewPath("httproute", httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx).Child("backendRefs").Index(backendIdx),
						fmt.Errorf("backend index %d exceeds available backends", backendIdx),
					))
					continue
				}

				backendRef := &httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs[backendIdx]

				if source.Ingress.Annotations[canaryAnnotation] == "true" {
					if canaryBackend != nil {
						errList = append(errList, field.Invalid(
							field.NewPath("httproute", httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
							fmt.Sprintf("ingresses %s/%s and %s/%s", canarySourceIngress.Namespace, canarySourceIngress.Name, source.Ingress.Namespace, source.Ingress.Name),
							"at most one canary backend is allowed per rule",
						))
						continue
					}

					config, err := parseCanaryConfig(source.Ingress)
					if err != nil {
						errList = append(errList, field.Invalid(
							field.NewPath("ingress", source.Ingress.Namespace, source.Ingress.Name, "metadata", "annotations"),
							source.Ingress.Annotations,
							fmt.Sprintf("failed to parse canary configuration: %v", err),
						))
						continue
					}

					canaryBackend = backendRef
					canaryConfig = config
					canarySourceIngress = source.Ingress
				} else {
					if nonCanaryBackend != nil {
						errList = append(errList, field.Invalid(
							field.NewPath("httproute", httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
							"multiple non-canary backends",
							"at most one non-canary backend is allowed per rule when using canary",
						))
						continue
					}
					nonCanaryBackend = backendRef
				}
			}

			// If there is a canary backend, validate and set weights
			if canaryBackend != nil {
				if nonCanaryBackend == nil {
					errList = append(errList, field.Invalid(
						field.NewPath("httproute", httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
						"canary backend without non-canary backend",
						"a non-canary backend is required when using canary",
					))
					continue
				}

				canaryWeight := canaryConfig.weight

				canaryBackend.Weight = &canaryWeight
				nonCanaryWeight := canaryConfig.weightTotal - canaryWeight
				nonCanaryBackend.Weight = &nonCanaryWeight

				notify(notifications.InfoNotification, fmt.Sprintf("parsed canary annotations of ingress %s/%s and set weights (canary: %d, non-canary: %d, total: %d)", 
					canarySourceIngress.Namespace, canarySourceIngress.Name, canaryWeight, nonCanaryWeight, canaryConfig.weightTotal), &httpRouteContext.HTTPRoute)
			}
		}
	}

	if len(errList) > 0 {
		return errList
	}
	return nil
}
