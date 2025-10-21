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
func parseCanaryConfig(ingress *networkingv1.Ingress) canaryConfig {
	config := canaryConfig{
		weight:      0,
		weightTotal: 100, // default
	}

	if weight := ingress.Annotations[canaryWeightAnnotation]; weight != "" {
		if w, err := strconv.Atoi(weight); err == nil {
			config.weight = int32(w)
		}
	}

	if total := ingress.Annotations[canaryWeightTotalAnnotation]; total != "" {
		if wt, err := strconv.Atoi(total); err == nil && wt > 0 {
			config.weightTotal = int32(wt)
		}
	}

	return config
}

func canaryFeature(ingresses []networkingv1.Ingress, servicePorts map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)

	for _, rg := range ruleGroups {
		key := types.NamespacedName{Namespace: rg.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
		httpRouteContext, ok := ir.HTTPRoutes[key]
		if !ok {
			continue
		}

		canaryEnabled := false
		for _, backendSources := range httpRouteContext.RuleBackendSources {
			for _, source := range backendSources {
				if source.Ingress != nil && source.Ingress.Annotations[canaryAnnotation] == "true" {
					canaryEnabled = true
					break
				}
			}
			if canaryEnabled {
				break
			}
		}

		if !canaryEnabled {
			continue
		}

		for ruleIdx, backendSources := range httpRouteContext.RuleBackendSources {
			if ruleIdx >= len(httpRouteContext.HTTPRoute.Spec.Rules) {
				continue
			}

			// There must be a non canary backend and at most one canary backend
			// This is done in place.
			var canaryBackend *gatewayv1.HTTPBackendRef
			var nonCanaryBackend *gatewayv1.HTTPBackendRef
			var canaryConfig canaryConfig
			
			for backendIdx, source := range backendSources {
				if source.Ingress == nil {
					continue
				}
				
				backendRef := &httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs[backendIdx]
				
				if source.Ingress.Annotations[canaryAnnotation] == "true" {
					if canaryBackend != nil {
						return field.ErrorList{
							field.Invalid(
								field.NewPath("httproute", httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
								"multiple canary backends",
								"at most one canary backend is allowed per rule",
							),
						}
					}
					canaryBackend = backendRef
					canaryConfig = parseCanaryConfig(source.Ingress)
				} else {
					if nonCanaryBackend != nil {
						return field.ErrorList{
							field.Invalid(
								field.NewPath("httproute", httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
								"multiple non-canary backends",
								"at most one non-canary backend is allowed per rule when using canary",
							),
						}
					}
					nonCanaryBackend = backendRef
				}
			}

			if canaryBackend != nil {
				// Cap canary weight at total
				canaryWeight := canaryConfig.weight
				if canaryWeight > canaryConfig.weightTotal {
					canaryWeight = canaryConfig.weightTotal
				}
				
				canaryBackend.Weight = &canaryWeight
				
				if nonCanaryBackend != nil {
					nonCanaryWeight := canaryConfig.weightTotal - canaryWeight
					nonCanaryBackend.Weight = &nonCanaryWeight
				}
			}
		}

		notify(notifications.InfoNotification, fmt.Sprintf("parsed canary annotations of ingress and patched %v fields", field.NewPath("httproute", "spec", "rules").Key("").Child("backendRefs")), &httpRouteContext.HTTPRoute)
	}

	return nil
}
