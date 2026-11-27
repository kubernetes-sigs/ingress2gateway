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
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func regexHosts(ingresses []networkingv1.Ingress) map[string]struct{} {
	hostsWithRegex := make(map[string]struct{})

	for _, ingress := range ingresses {
		val := ingress.Annotations[CanaryAnnotation]
		isCanary, _ := strconv.ParseBool(val)
		if isCanary {
			continue
		}
		useRegex, _ := strconv.ParseBool(ingress.Annotations[UseRegexAnnotation])
		hasRewriteTarget := ingress.Annotations[RewriteTargetAnnotation] != ""
		if !useRegex && !hasRewriteTarget {
			continue
		}

		for _, rule := range ingress.Spec.Rules {
			if rule.Host != "" {
				hostsWithRegex[rule.Host] = struct{}{}
			}
		}
	}
	return hostsWithRegex
}

// regexFeature converts ingress-nginx regex-driving annotations
// to Gateway API HTTPRoute RegularExpression path match.
func regexFeature(_ notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	hostsWithRegex := regexHosts(ingresses)

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
				if path.Path != nil {
					// Ingress nginx regex path matches are prefix matches by default
					httpRouteCtx.Spec.Rules[i].Matches[j].Path.Type = ptr.To[gatewayv1.PathMatchType](gatewayv1.PathMatchRegularExpression)
					// The inline flag gives case-insensitive matching. Support varies by implementation.
					httpRouteCtx.Spec.Rules[i].Matches[j].Path.Value = ptr.To("(?i)" + *httpRouteCtx.Spec.Rules[i].Matches[j].Path.Value + ".*")
				}
			}
		}
	}
	return nil
}

func warnRegexCompatibility(notify notifications.NotifyFunc, ir *providerir.ProviderIR) {
	for _, httpRouteCtx := range ir.HTTPRoutes {
		if !hasCaseInsensitiveRegexPath(httpRouteCtx.Spec.Rules) {
			continue
		}
		notify(notifications.WarningNotification,
			"Case-insensitive regex path matches use the (?i) flag. Support and accepted syntax vary by Gateway implementation. Please verify the generated paths against your target implementation.",
			&httpRouteCtx.HTTPRoute)
	}
}

func hasCaseInsensitiveRegexPath(rules []gatewayv1.HTTPRouteRule) bool {
	for _, rule := range rules {
		for _, match := range rule.Matches {
			path := match.Path
			if path != nil && ptr.Deref(path.Type, "") == gatewayv1.PathMatchRegularExpression &&
				strings.HasPrefix(ptr.Deref(path.Value, ""), "(?i)") {
				return true
			}
		}
	}
	return false
}
