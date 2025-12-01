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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const clientBodyBufferSizeAnnotation = "nginx.ingress.kubernetes.io/client-body-buffer-size"

// bodyBufferFeature parses the "nginx.ingress.kubernetes.io/client-body-buffer-size" annotation
// from Ingresses and records them as extension settings in HTTPRoute IR.
func bodyBufferFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errList field.ErrorList

	// Parse buffer size annotations from all ingresses
	bufferByIngress := make(map[types.NamespacedName]*resource.Quantity)
	for _, ing := range ingresses {
		val := ing.Annotations[clientBodyBufferSizeAnnotation]
		if val == "" {
			continue
		}

		quantity, err := resource.ParseQuantity(val)
		if err != nil {
			errList = append(errList, field.Invalid(
				field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(clientBodyBufferSizeAnnotation),
				val,
				fmt.Sprintf("failed to parse client-body-buffer-size: %v", err),
			))
			continue
		}

		bufferByIngress[types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}] = &quantity
	}

	// Early return if no buffer annotations found
	if len(bufferByIngress) == 0 {
		return errList
	}

	// Apply buffer settings to HTTPRoutes at the rule level using RuleBackendSources.
	// Each rule tracks which Ingress contributed its backends, allowing us to apply
	// buffer settings on a per-rule basis using Policy sectionName.
	//
	// When all rules have the same buffer setting, it is applied to the entire HTTPRoute
	// (using IndexAttachAllRules/-1). Otherwise, settings are applied per rule.
	for routeKey, httpRouteContext := range ir.HTTPRoutes {
		// Collect buffer settings per rule
		ruleBufferSettings := make(map[int]*resource.Quantity)

		for ruleIdx, backendSources := range httpRouteContext.RuleBackendSources {
			if ruleIdx >= len(httpRouteContext.HTTPRoute.Spec.Rules) {
				errList = append(errList, field.InternalError(
					field.NewPath("httproute", httpRouteContext.HTTPRoute.Namespace, httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx),
					fmt.Errorf("rule index %d exceeds available rules", ruleIdx),
				))
				continue
			}

			// Collect buffer settings from all ingresses contributing to this rule
			bufferSettings := make(map[string]*resource.Quantity)
			for _, source := range backendSources {
				if source.Ingress == nil {
					continue
				}

				ingKey := types.NamespacedName{
					Namespace: source.Ingress.Namespace,
					Name:      source.Ingress.Name,
				}
				if bufferSize, exists := bufferByIngress[ingKey]; exists {
					bufferSettings[bufferSize.String()] = bufferSize
				}
			}

			// Skip if no buffer settings for this rule
			if len(bufferSettings) == 0 {
				continue
			}

			if len(bufferSettings) > 1 {
				errList = append(errList, field.Invalid(
					field.NewPath("httproute", httpRouteContext.HTTPRoute.Namespace, httpRouteContext.HTTPRoute.Name, "spec", "rules").Index(ruleIdx),
					"multiple different buffer settings",
					"backends from different ingresses with different client-body-buffer-size annotations are in the same rule",
				))
				continue
			}

			// Record buffer setting for this rule
			for _, bufferSize := range bufferSettings {
				ruleBufferSettings[ruleIdx] = bufferSize
				break
			}
		}

		// Optimize: if all rules have the same buffer setting, apply to entire HTTPRoute
		if len(ruleBufferSettings) > 0 {
			allAttached := len(ruleBufferSettings) == len(httpRouteContext.HTTPRoute.Spec.Rules)

			// Check if all buffer settings have the same value
			var firstBuffer *resource.Quantity
			allSameValue := true
			for _, buffer := range ruleBufferSettings {
				if firstBuffer == nil {
					firstBuffer = buffer
				} else if buffer.String() != firstBuffer.String() {
					allSameValue = false
					break
				}
			}

			// Initialize map if needed
			if httpRouteContext.ExtensionSettings == nil {
				httpRouteContext.ExtensionSettings =
					make(map[intermediate.RouteSettingsAttachment]*intermediate.HTTPRouteExtensionSetting)
			}

			if allAttached && allSameValue {
				// All rules have the same buffer setting - apply to entire HTTPRoute
				idx := intermediate.RouteSettingsAttachment{
					Rule:    intermediate.IndexAttachAllRules,
					Backend: intermediate.IndexAttachAllBackends,
				}
				if _, ok := httpRouteContext.ExtensionSettings[idx]; !ok {
					httpRouteContext.ExtensionSettings[idx] = &intermediate.HTTPRouteExtensionSetting{
						ProcessingStatus: make(map[intermediate.ExtensionFeature]*intermediate.ExtensionSettingMetadata),
					}
				}
				httpRouteContext.ExtensionSettings[idx].Buffer = firstBuffer
				httpRouteContext.ExtensionSettings[idx].ProcessingStatus[intermediate.ExtensionFeatureBodyBuffer] =
					&intermediate.ExtensionSettingMetadata{
						Provider: Name,
					}

				notify(notifications.InfoNotification,
					fmt.Sprintf("set client-body-buffer-size %s to all rules of HTTPRoute %s/%s",
						firstBuffer.String(), routeKey.Namespace, routeKey.Name),
					&httpRouteContext.HTTPRoute)
			} else {
				// Different settings per rule - apply individually
				for ruleIdx, bufferSize := range ruleBufferSettings {
					idx := intermediate.RouteSettingsAttachment{
						Rule:    ruleIdx,
						Backend: intermediate.IndexAttachAllBackends,
					}
					if _, ok := httpRouteContext.ExtensionSettings[idx]; !ok {
						httpRouteContext.ExtensionSettings[idx] = &intermediate.HTTPRouteExtensionSetting{
							ProcessingStatus: make(map[intermediate.ExtensionFeature]*intermediate.ExtensionSettingMetadata),
						}
					}
					httpRouteContext.ExtensionSettings[idx].Buffer = bufferSize
					httpRouteContext.ExtensionSettings[idx].ProcessingStatus[intermediate.ExtensionFeatureBodyBuffer] =
						&intermediate.ExtensionSettingMetadata{
							Provider: Name,
						}

					notify(notifications.InfoNotification,
						fmt.Sprintf("set client-body-buffer-size %s to rule %d of HTTPRoute %s/%s",
							bufferSize.String(), ruleIdx, routeKey.Namespace, routeKey.Name),
						&httpRouteContext.HTTPRoute)
				}
			}
		}

		// Write back updated context into the IR
		ir.HTTPRoutes[routeKey] = httpRouteContext
	}

	return errList
}
