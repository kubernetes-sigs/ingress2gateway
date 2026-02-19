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

package ingressnginx

import (
	"strconv"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// regexFeature converts the "nginx.ingress.kubernetes.io/use-regex" annotation
// to Gateway API HTTPRoute RegularExpression path match.
func regexFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	for _, httpRouteCtx := range ir.HTTPRoutes {
		for ruleIdx, rule := range httpRouteCtx.Spec.Rules {
			if ruleIdx >= len(httpRouteCtx.RuleBackendSources) {
				continue
			}
			sources := httpRouteCtx.RuleBackendSources[ruleIdx]
			if len(sources) == 0 {
				continue
			}

			// Check if the source ingress has the regex annotation enabled.
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			val, ok := ingress.Annotations[UseRegexAnnotation]
			if !ok {
				continue
			}

			useRegex, err := strconv.ParseBool(val)
			if err != nil {
				// Invalid boolean value, default to false or log invalid usage?
				// For now, let's treat invalid as false (safe default).
				// We definitely shouldn't panic or crash.
				continue
			}

			if useRegex {
				for matchIdx := range rule.Matches {
					path := rule.Matches[matchIdx].Path
					if path != nil {
						val := gatewayv1.PathMatchRegularExpression
						path.Type = &val
					}
				}
			}
		}
	}
	return errs
}
