/*
Copyright The Kubernetes Authors.

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

package nginx

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

// emitClientSettingsPolicies emits a route-scoped ClientSettingsPolicy
func (e *Emitter) emitClientSettingsPolicies(ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) {
	routeKeys := make([]types.NamespacedName, 0, len(ir.HTTPRoutes))
	for key := range ir.HTTPRoutes {
		routeKeys = append(routeKeys, key)
	}

	sort.Slice(routeKeys, func(i, j int) bool {
		if routeKeys[i].Namespace == routeKeys[j].Namespace {
			return routeKeys[i].Name < routeKeys[j].Name
		}
		return routeKeys[i].Namespace < routeKeys[j].Namespace
	})

	for _, routeKey := range routeKeys {
		route := ir.HTTPRoutes[routeKey]

		if len(route.BodySizeByRuleIdx) == 0 {
			continue
		}

		ruleIndexes := make([]int, 0, len(route.BodySizeByRuleIdx))
		for ruleIdx := range route.BodySizeByRuleIdx {
			ruleIndexes = append(ruleIndexes, ruleIdx)
		}
		// sorting makes output deterministic for testing
		sort.Ints(ruleIndexes)

		var maxSize *resource.Quantity
		valid := true
		hasUnsupportedBufferSize := false

		for _, ruleIdx := range ruleIndexes {
			if ruleIdx < 0 || ruleIdx >= len(route.Spec.Rules) {
				valid = false
				break
			}

			bodySize := route.BodySizeByRuleIdx[ruleIdx]
			if bodySize == nil || bodySize.MaxSize == nil {
				valid = false
				break
			}
			if maxSize == nil {
				maxSize = bodySize.MaxSize
				continue
			}

			if !maxSize.Equal(*bodySize.MaxSize) {
				valid = false
				break
			}
		}

		for _, bodySize := range route.BodySizeByRuleIdx {
			if bodySize != nil && bodySize.BufferSize != nil {
				hasUnsupportedBufferSize = true
				break
			}
		}

		if hasUnsupportedBufferSize {
			e.notify(
				notifications.WarningNotification,
				"skipping ClientSettingsPolicy as body buffer size is not supported",
				&route.HTTPRoute,
			)
			continue
		}

		if !valid || maxSize == nil {
			e.notify(
				notifications.ErrorNotification,
				"skipping ClientSettingsPolicy as HTTPRoute rules have incomplete or conflicting body size",
				&route.HTTPRoute,
			)
			continue
		}

		policyName := fmt.Sprintf("%s-client-settings", route.Name)
		policy := EmitClientSettings(
			policyName,
			route.Namespace,
			route.Name,
			maxSize.String(),
		)

		object, err := i2gw.CastToUnstructured(policy)
		if err != nil {
			e.notify(
				notifications.ErrorNotification,
				fmt.Sprintf("failed to convert ClientSettingsPolicy %s: %v", policyName, err),
				policy,
			)
			continue
		}

		gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *object)

		// mark successfully emitted as processed
		fullyProcessed := true
		for _, bodySize := range route.BodySizeByRuleIdx {
			if bodySize != nil && bodySize.BufferSize != nil {
				fullyProcessed = false
				break
			}
		}

		if fullyProcessed {
			route.BodySizeByRuleIdx = nil
			ir.HTTPRoutes[routeKey] = route
		}
	}
}
