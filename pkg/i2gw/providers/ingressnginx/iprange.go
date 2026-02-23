/*
Copyright 2026 The Kubernetes Authors.

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
	"strings"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func parseIPSourceRangeAnnotation(annotations map[string]string, key string) []string {
	value, ok := annotations[key]
	if !ok {
		return nil
	}
	items := strings.Split(value, ",")
	for idx, item := range items {
		items[idx] = strings.TrimSpace(item)
	}
	return items
}

func applyIPRangeControlToEmitterIR(pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) {
	for key, pRouteCtx := range pIR.HTTPRoutes {
		eRouteCtx, ok := eIR.HTTPRoutes[key]
		if !ok {
			continue
		}

		for ruleIdx := range pRouteCtx.HTTPRoute.Spec.Rules {
			if ruleIdx >= len(pRouteCtx.RuleBackendSources) {
				continue
			}
			sources := pRouteCtx.RuleBackendSources[ruleIdx]

			ing := getNonCanaryIngress(sources)
			if ing == nil {
				continue
			}

			var allowList, denyList []string

			allowList = parseIPSourceRangeAnnotation(ing.Annotations, WhiteListSourceRangeAnnotation)
			denyList = parseIPSourceRangeAnnotation(ing.Annotations, DenyListSourceRangeAnnotation)

			if len(allowList) == 0 && len(denyList) == 0 {
				continue
			}

			if eRouteCtx.IPRangeControlByRuleIdx == nil {
				eRouteCtx.IPRangeControlByRuleIdx = make(map[int]*emitterir.IPRangeControl)
			}

			ipRangeControl := &emitterir.IPRangeControl{
				AllowList: allowList,
				DenyList:  denyList,
			}
			{
				source := fmt.Sprintf("%s/%s", ing.Name, ing.Namespace)
				message := "IP-based authorization is not supported"
				paths := make([]*field.Path, 2)
				if len(allowList) > 0 {
					paths = append(paths, field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations", WhiteListSourceRangeAnnotation))
				}
				if len(denyList) > 0 {
					paths = append(paths, field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations", DenyListSourceRangeAnnotation))
				}
				ipRangeControl.Metadata = emitterir.NewExtensionFeatureMetadata(
					source,
					paths,
					message,
				)
			}

			if len(allowList) > 0 {
				notify(notifications.InfoNotification, fmt.Sprintf("parsed whitelist-source-range annotation of ingress %s/%s: allowing CIDRs %s for HTTPRoute rule index %d",
					ing.Namespace, ing.Name, strings.Join(allowList, ", "), ruleIdx), &eRouteCtx.HTTPRoute)
			}
			if len(denyList) > 0 {
				notify(notifications.InfoNotification, fmt.Sprintf("parsed denylist-source-range annotation of ingress %s/%s: denying CIDRs %s for HTTPRoute rule index %d",
					ing.Namespace, ing.Name, strings.Join(denyList, ", "), ruleIdx), &eRouteCtx.HTTPRoute)
			}
		}
		eIR.HTTPRoutes[key] = eRouteCtx
	}
}
