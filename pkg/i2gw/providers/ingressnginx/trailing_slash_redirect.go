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
	"strings"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func applyTrailingSlashPathRedirectsToEmitterIR(eir *emitterir.EmitterIR) {
	for key, routeCtx := range eir.HTTPRoutes {
		rules := routeCtx.Spec.Rules
		exactPathRuleExists := make(map[string]struct{})
		for _, rule := range rules {
			for _, match := range rule.Matches {
				if match.Path == nil || match.Path.Type == nil || match.Path.Value == nil {
					continue
				}
				if *match.Path.Type == gatewayv1.PathMatchExact {
					exactPathRuleExists[*match.Path.Value] = struct{}{}
				}
			}
		}

		redirectAdded := make(map[string]struct{})
		for _, rule := range rules {
			for _, match := range rule.Matches {
				if match.Path == nil || match.Path.Type == nil || match.Path.Value == nil {
					continue
				}

				matchType := *match.Path.Type
				if matchType != gatewayv1.PathMatchExact && matchType != gatewayv1.PathMatchPathPrefix {
					continue
				}

				redirectTarget := *match.Path.Value
				if redirectTarget == "/" || !strings.HasSuffix(redirectTarget, "/") {
					continue
				}

				redirectSource := strings.TrimSuffix(redirectTarget, "/")
				if redirectSource == "" {
					continue
				}
				if _, exists := exactPathRuleExists[redirectSource]; exists {
					continue
				}
				if _, added := redirectAdded[redirectSource]; added {
					continue
				}

				exact := gatewayv1.PathMatchExact
				routeCtx.Spec.Rules = append(routeCtx.Spec.Rules, gatewayv1.HTTPRouteRule{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &exact,
								Value: ptr.To(redirectSource),
							},
						},
					},
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
								StatusCode: ptr.To(301),
								Path: &gatewayv1.HTTPPathModifier{
									Type:            gatewayv1.FullPathHTTPPathModifier,
									ReplaceFullPath: ptr.To(redirectTarget),
								},
							},
						},
					},
				})

				redirectAdded[redirectSource] = struct{}{}
			}
		}

		eir.HTTPRoutes[key] = routeCtx
	}
}
