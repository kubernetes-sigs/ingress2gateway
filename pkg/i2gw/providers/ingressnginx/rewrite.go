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
	"regexp"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// regex for detecting common prefix strip usage of rewrite-target
// e.g. path: /foo(/|$)(.*) and rewrite-target: /$2
var prefixStripRegex = regexp.MustCompile(`^(.*)\(/\|\$\)\(\.\*\)$`)

func rewriteFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
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

			// Check if any source ingress has the rewrite annotation.
			// We take the last one found if multiple sources have conflicting annotations (arbitrary rule, but standard in i2gw)
			var rewriteTarget string
			var ingress *networkingv1.Ingress
			for _, source := range sources {
				if val, ok := source.Ingress.Annotations[RewriteTargetAnnotation]; ok {
					rewriteTarget = val
					ingress = source.Ingress
				}
			}

			if rewriteTarget == "" {
				continue
			}

			for _, match := range rule.Matches {
				path := match.Path
				if path == nil || path.Value == nil {
					continue
				}

				pathVal := *path.Value
				
				// Case 1: Prefix Strip Pattern
				// Check if path matches /foo(/|$)(.*)
				prefixMatch := prefixStripRegex.FindStringSubmatch(pathVal)
				if len(prefixMatch) == 2 {
					// It captures the prefix in group 1.
					// e.g. /foo(/|$)(.*) -> prefix is /foo
					
					// If rewrite target is /$2 (or /$1 depending on how many groups earlier? Nginx usually starts from 1)
					// Standard Nginx regex groups:
					// location ~ ^/foo(/|$)(.*) { rewrite ^/foo(/|$)(.*) /$2 break; }
					// So /$2 refers to the (.*) part.
					
					if rewriteTarget == "/$2" {
						// This is a clear indicator of "Strip Prefix /foo and replace with /"
						// Gateway API ReplacePrefixMatch:
						// If path is /foo/bar, we want to replace /foo with /.
						
						// Note: Nginx's /$2 effectively means "take the suffix".
						// Gateway API replacePrefixMatch says "replace the matched prefix with this string".
						// So replacing /foo with / results in /bar (if URI was /foo/bar) matches? No wait.
						
						// ReplacePrefixMatch:
						// "Specifies matching path prefix that should be replaced."
						// "matches.path.value IS the prefix." 
						// BUT here our matches.path.value is likely converted to Regex /foo(/|$)(.*) because of use-regex.
						
						// If we want to use ReplacePrefixMatch, the MATCH type must be PathPrefix.
						// But PR2 converts it to RegularExpression. 
						// Gateway API URLRewrite filter works with any match type, IF supported by implementation.
						
						// However, ReplacePrefixMatch usually requires PathPrefix match in standard implementations?
						// Let's check spec.
						// "ReplacePrefixMatch is only compatible with a PathPrefix match..." - Actually spec says:
						// "If the Path Match is not PathPrefix, invalid." (Wait, checking spec...)
						
						// I better verify this expectation.
						// Use ReplaceFullPath if we can't be sure.
						
						// Actually, if we are rewriting to /$2, we effectively want to strip the prefix found before the (.*).
						// If the path was converted to Regex /foo(/|$)(.*), we can't easily use ReplacePrefixMatch on the regex itself.
						
						// PROPOSAL:
						// If we detect this pattern:
						// 1. Change the Match from Regex back to Prefix: /foo
						// 2. Add URLRewrite with ReplacePrefixMatch: /
						
						// This is MUCH cleaner for Gateway API than keeping it as Regex.
						// Let's implement this "Upgrade to Prefix" logic.
						
						prefix := prefixMatch[1] // /foo
						
						// Update the match to be Prefix Match
						prefixType := gatewayv1.PathMatchPathPrefix
						path.Type = &prefixType
						path.Value = &prefix
						
						// Add the filter
						replacePrefix := "/"
						filter := gatewayv1.HTTPRouteFilter{
							Type: gatewayv1.HTTPRouteFilterURLRewrite,
							URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
								Path: &gatewayv1.HTTPPathModifier{
									Type: gatewayv1.PrefixMatchHTTPPathModifier,
									ReplacePrefixMatch: &replacePrefix,
								},
							},
						}
						httpRouteCtx.HTTPRoute.Spec.Rules[ruleIdx].Filters = append(httpRouteCtx.HTTPRoute.Spec.Rules[ruleIdx].Filters, filter)
						continue
					}
				}

				// Case 2: Static Replacement
				// If rewrite-target does not contain capture groups ($1, $2, etc.)
				isStatic, _ := regexp.MatchString(`\$\d+`, rewriteTarget)
				if !isStatic {
					// Static replacement
					// e.g. rewrite-target: /new-path
					// Gateway API ReplaceFullPath
					
					filter := gatewayv1.HTTPRouteFilter{
						Type: gatewayv1.HTTPRouteFilterURLRewrite,
						URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
							Path: &gatewayv1.HTTPPathModifier{
								Type: gatewayv1.FullPathHTTPPathModifier,
								ReplaceFullPath: &rewriteTarget,
							},
						},
					}
					httpRouteCtx.HTTPRoute.Spec.Rules[ruleIdx].Filters = append(httpRouteCtx.HTTPRoute.Spec.Rules[ruleIdx].Filters, filter)
					continue
				}

				// Case 3: Unsupported
				notify(notifications.WarningNotification, fmt.Sprintf("Ingress %s/%s uses unsupported complex rewrite-target '%s'. Only static rewrites or simple prefix stripping (regex + /$2) are supported.", ingress.Namespace, ingress.Name, rewriteTarget), &httpRouteCtx.HTTPRoute)
			}
		}
	}
	return errs
}
