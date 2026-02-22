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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// canaryConfig holds the parsed canary configuration from a single Ingress
type canaryConfig struct {
	isHeader    bool
	header      string
	headerValue string
	isWeight    bool
	weight      int32
	weightTotal int32
}

// parseCanaryConfig extracts canary weight configuration from an Ingress
func parseCanaryConfig(ingress *networkingv1.Ingress) (canaryConfig, error) {
	config := canaryConfig{
		weight:      0,
		weightTotal: 100, // default
	}

	if ingress.Annotations[CanaryByHeaderPattern] != "" {
		notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
			ingress.Namespace, ingress.Name, CanaryByHeaderPattern), ingress)
	}

	if ingress.Annotations[CanaryByCookie] != "" {
		notify(notifications.WarningNotification, fmt.Sprintf("ingress %s/%s uses unsupported annotation %s",
			ingress.Namespace, ingress.Name, CanaryByCookie), ingress)
	}

	if ingress.Annotations[CanaryByHeader] != "" {
		config.isHeader = true
	}
	config.header = ingress.Annotations[CanaryByHeader]
	config.headerValue = ingress.Annotations[CanaryByHeaderValue]

	weight := ingress.Annotations[CanaryWeightAnnotation]

	if weight != "" {
		w, err := strconv.ParseInt(weight, 10, 32)
		if err != nil {
			return config, fmt.Errorf("invalid canary-weight annotation %q: %w", weight, err)
		}
		if w < 0 {
			return config, fmt.Errorf("canary-weight must be non-negative, got %d", w)
		}
		config.isWeight = true
		config.weight = int32(w)
	}

	if total := ingress.Annotations[CanaryWeightTotalAnnotation]; total != "" {
		wt, err := strconv.ParseInt(total, 10, 32)
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

func canaryFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errList field.ErrorList

	for _, rg := range ruleGroups {
		key := types.NamespacedName{Namespace: rg.Namespace, Name: common.RouteName(rg.Name, rg.Host)}

		if httpRouteContext, ok := ir.HTTPRoutes[key]; ok {
			var rulesToAdd []gatewayv1.HTTPRouteRule
			var sourcesToAdd [][]providerir.BackendSource

			for ruleIdx := 0; ruleIdx < len(httpRouteContext.HTTPRoute.Spec.Rules); ruleIdx++ {
				backendSources := httpRouteContext.RuleBackendSources[ruleIdx]

				canaryWeight, nonCanaryWeight, config, canarySourceIngress, canaryBackendIdx, nonCanaryBackendIdx, parseErrs := getCanaryInfo(backendSources, "httproute", httpRouteContext.HTTPRoute.Name, ruleIdx)
				errList = append(errList, parseErrs...)
				if canaryBackendIdx != -1 && nonCanaryBackendIdx != -1 {
					// Set weights if isWeight is true or both header and weight are not set (all traffic should go to non-canary)
					if config.isWeight || !config.isHeader {
						httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs[canaryBackendIdx].Weight = &canaryWeight
						httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs[nonCanaryBackendIdx].Weight = &nonCanaryWeight
						notify(notifications.InfoNotification, fmt.Sprintf("parsed canary annotations of ingress %s/%s and set weights (canary: %d, non-canary: %d, total: %d)",
							canarySourceIngress.Namespace, canarySourceIngress.Name, canaryWeight, nonCanaryWeight, config.weightTotal), &httpRouteContext.HTTPRoute)
					}

					if config.isHeader {
						canaryBackend := httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs[canaryBackendIdx]
						nonCanaryBackend := httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs[nonCanaryBackendIdx]
						canaryBackendSource := backendSources[canaryBackendIdx]
						nonCanaryBackendSource := backendSources[nonCanaryBackendIdx]

						var header = "always"
						if config.headerValue != "" {
							header = config.headerValue
						}
						canaryBackendCopy := canaryBackend
						canaryBackendCopy.Weight = nil
						newRule := gatewayv1.HTTPRouteRule{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Headers: []gatewayv1.HTTPHeaderMatch{
										{
											Name:  gatewayv1.HTTPHeaderName(config.header),
											Value: header,
										},
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{canaryBackendCopy},
						}
						rulesToAdd = append(rulesToAdd, newRule)
						sourcesToAdd = append(sourcesToAdd, []providerir.BackendSource{canaryBackendSource, nonCanaryBackendSource})

						if config.headerValue == "" {
							nonCanaryBackendCopy := nonCanaryBackend
							nonCanaryBackendCopy.Weight = nil
							neverRule := gatewayv1.HTTPRouteRule{
								Matches: []gatewayv1.HTTPRouteMatch{
									{
										Headers: []gatewayv1.HTTPHeaderMatch{
											{
												Name:  gatewayv1.HTTPHeaderName(config.header),
												Value: "never",
											},
										},
									},
								},
								BackendRefs: []gatewayv1.HTTPBackendRef{nonCanaryBackendCopy},
							}
							rulesToAdd = append(rulesToAdd, neverRule)
							sourcesToAdd = append(sourcesToAdd, []providerir.BackendSource{nonCanaryBackendSource})

							notify(notifications.InfoNotification, fmt.Sprintf("parsed canary annotations of ingress %s/%s and set header \"%s\" with value \"never\" for non-canary backend",
								canarySourceIngress.Namespace, canarySourceIngress.Name, config.header), &httpRouteContext.HTTPRoute)
						}

						if !config.isWeight {
							// Find and remove the canary backend from the original rule's BackendRefs
							var filteredBackendRefs []gatewayv1.HTTPBackendRef
							var filteredBackendSources []providerir.BackendSource
							for i := range httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs {
								if i != canaryBackendIdx {
									filteredBackendRefs = append(filteredBackendRefs, httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs[i])
									filteredBackendSources = append(filteredBackendSources, backendSources[i])
								}
							}
							httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx].BackendRefs = filteredBackendRefs
							httpRouteContext.RuleBackendSources[ruleIdx] = filteredBackendSources
						}
						notify(notifications.InfoNotification, fmt.Sprintf("parsed canary annotations of ingress %s/%s and set header \"%s\" with value \"%s\"",
							canarySourceIngress.Namespace, canarySourceIngress.Name, config.header, config.headerValue), &httpRouteContext.HTTPRoute)
					}
				}
			}
			httpRouteContext.HTTPRoute.Spec.Rules = append(httpRouteContext.HTTPRoute.Spec.Rules, rulesToAdd...)
			httpRouteContext.RuleBackendSources = append(httpRouteContext.RuleBackendSources, sourcesToAdd...)
			ir.HTTPRoutes[key] = httpRouteContext
		}

		if grpcRouteContext, ok := ir.GRPCRoutes[key]; ok {
			for ruleIdx, backendSources := range grpcRouteContext.RuleBackendSources {
				if ruleIdx >= len(grpcRouteContext.GRPCRoute.Spec.Rules) {
					errList = append(errList, field.InternalError(
						field.NewPath("grpcroute", grpcRouteContext.GRPCRoute.Name, "spec", "rules").Index(ruleIdx),
						fmt.Errorf("rule index %d exceeds available rules", ruleIdx),
					))
					continue
				}

				canaryWeight, nonCanaryWeight, config, canarySourceIngress, canaryBackendIdx, nonCanaryBackendIdx, parseErrs := getCanaryInfo(backendSources, "grpcroute", grpcRouteContext.GRPCRoute.Name, ruleIdx)
				errList = append(errList, parseErrs...)
				if canaryBackendIdx != -1 && nonCanaryBackendIdx != -1 {
					grpcRouteContext.GRPCRoute.Spec.Rules[ruleIdx].BackendRefs[canaryBackendIdx].Weight = &canaryWeight
					grpcRouteContext.GRPCRoute.Spec.Rules[ruleIdx].BackendRefs[nonCanaryBackendIdx].Weight = &nonCanaryWeight
					notify(notifications.InfoNotification, fmt.Sprintf("parsed canary annotations of ingress %s/%s and set weights (canary: %d, non-canary: %d, total: %d)",
						canarySourceIngress.Namespace, canarySourceIngress.Name, canaryWeight, nonCanaryWeight, config.weightTotal), &grpcRouteContext.GRPCRoute)
				}
			}
		}
	}

	return errList
}

func getCanaryInfo(backendSources []providerir.BackendSource, routeType, routeName string, ruleIdx int) (int32, int32, canaryConfig, *networkingv1.Ingress, int, int, field.ErrorList) {
	var errList field.ErrorList
	canaryBackendIdx := -1
	nonCanaryBackendIdx := -1
	var config canaryConfig
	var canarySourceIngress *networkingv1.Ingress

	for backendIdx, source := range backendSources {
		if source.Ingress == nil {
			continue
		}

		if source.Ingress.Annotations[CanaryAnnotation] == "true" {
			if canaryBackendIdx != -1 {
				errList = append(errList, field.Invalid(
					field.NewPath(routeType, routeName, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
					"multiple canary backends",
					"at most one canary backend is allowed per rule",
				))
				continue
			}

			parsedConfig, err := parseCanaryConfig(source.Ingress)
			if err != nil {
				errList = append(errList, field.Invalid(
					field.NewPath("ingress", source.Ingress.Namespace, source.Ingress.Name, "metadata", "annotations"),
					source.Ingress.Annotations,
					fmt.Sprintf("failed to parse canary configuration: %v", err),
				))
				continue
			}

			canaryBackendIdx = backendIdx
			config = parsedConfig
			canarySourceIngress = source.Ingress
		} else {
			if nonCanaryBackendIdx != -1 {
				errList = append(errList, field.Invalid(
					field.NewPath(routeType, routeName, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
					"multiple non-canary backends",
					"at most one non-canary backend is allowed per rule when using canary",
				))
				continue
			}
			nonCanaryBackendIdx = backendIdx
		}
	}

	if canaryBackendIdx != -1 {
		if nonCanaryBackendIdx == -1 {
			errList = append(errList, field.Invalid(
				field.NewPath(routeType, routeName, "spec", "rules").Index(ruleIdx).Child("backendRefs"),
				"canary backend without non-canary backend",
				"a non-canary backend is required when using canary",
			))
			return 0, 0, config, nil, -1, -1, errList
		}
		canaryWeight := config.weight
		nonCanaryWeight := config.weightTotal - canaryWeight
		return canaryWeight, nonCanaryWeight, config, canarySourceIngress, canaryBackendIdx, nonCanaryBackendIdx, errList
	}

	return 0, 0, config, nil, -1, -1, errList
}
