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
	"strconv"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// regexFeature converts the "nginx.ingress.kubernetes.io/use-regex" annotation
// to Gateway API HTTPRoute RegularExpression path match.
func regexFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	hostsWithRegex := make(map[string]struct{})

	for _, ingress := range ingresses {
		// TODO if there is a rewrite-target annotation, the path is treated as a regex even if use-regex is not set to true. We should also check that.
		val, ok := ingress.Annotations[UseRegexAnnotation]
		if !ok {
			continue
		}
		useRegex, _ := strconv.ParseBool(val)
		if !useRegex {
			continue
		}
		val = ingress.Annotations[CanaryAnnotation]
		isCanary, _ := strconv.ParseBool(val)
		if isCanary {
			continue
		}

		for _, rule := range ingress.Spec.Rules {
			if rule.Host != "" {
				hostsWithRegex[rule.Host] = struct{}{}
			}
		}
	}

	for _, httpRouteCtx := range ir.HTTPRoutes {
		hasRegex := false
		for _, host := range httpRouteCtx.Spec.Hostnames {
			if _, found := hostsWithRegex[string(host)]; found {
				hasRegex = true
				break
			}
		}
		if !hasRegex {
			continue
		}

		for i, rule := range httpRouteCtx.Spec.Rules {
			for j, path := range rule.Matches {
				if path.Path != nil  {
					// Ingress nginx regex path matches are prefix matches by default
					httpRouteCtx.Spec.Rules[i].Matches[j].Path.Type = ptr.To[gatewayv1.PathMatchType](gatewayv1.PathMatchRegularExpression)
					httpRouteCtx.Spec.Rules[i].Matches[j].Path.Value = ptr.To(*httpRouteCtx.Spec.Rules[i].Matches[j].Path.Value + ".*")
					// TODO. They are also case insensitive.
				}
			}
		}
	}
	return errs
}
