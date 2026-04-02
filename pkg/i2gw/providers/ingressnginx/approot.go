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
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// appRootFeature converts the nginx.ingress.kubernetes.io/app-root annotation
// to a Gateway API RequestRedirect filter. The annotation causes ingress-nginx
// to return a 302 redirect from "/" to the specified path.
//
// In Gateway API terms, this adds a new HTTPRouteRule with an Exact "/"
// path match and a RequestRedirect filter, while keeping the existing
// PathPrefix "/" rule intact for other request paths.
func appRootFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	for key, httpRouteContext := range ir.HTTPRoutes {
		var appRootPath string

		for ruleIndex := range httpRouteContext.HTTPRoute.Spec.Rules {
			if ruleIndex >= len(httpRouteContext.RuleBackendSources) {
				continue
			}

			ingress := getNonCanaryIngress(httpRouteContext.RuleBackendSources[ruleIndex])
			if ingress == nil {
				continue
			}

			appRoot, ok := ingress.Annotations[AppRootAnnotation]
			if !ok || appRoot == "" {
				continue
			}

			// Only apply app-root when the rule matches PathPrefix "/".
			for _, match := range httpRouteContext.HTTPRoute.Spec.Rules[ruleIndex].Matches {
				if match.Path != nil &&
					match.Path.Type != nil && *match.Path.Type == gatewayv1.PathMatchPathPrefix &&
					match.Path.Value != nil && *match.Path.Value == "/" {
					appRootPath = appRoot
					break
				}
			}

			if appRootPath != "" {
				break
			}
		}

		if appRootPath == "" {
			continue
		}

		notify(notifications.InfoNotification, fmt.Sprintf("Translating app-root annotation to a redirect from / to %s", appRootPath))

		redirectRule := gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  ptr.To(gatewayv1.PathMatchExact),
						Value: ptr.To("/"),
					},
				},
			},
			Filters: []gatewayv1.HTTPRouteFilter{
				{
					Type: gatewayv1.HTTPRouteFilterRequestRedirect,
					RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
						Path: &gatewayv1.HTTPPathModifier{
							Type:            gatewayv1.FullPathHTTPPathModifier,
							ReplaceFullPath: ptr.To(appRootPath),
						},
						StatusCode: ptr.To(302),
					},
				},
			},
		}

		httpRouteContext.HTTPRoute.Spec.Rules = append(httpRouteContext.HTTPRoute.Spec.Rules, redirectRule)
		httpRouteContext.RuleBackendSources = append(httpRouteContext.RuleBackendSources, nil)

		ir.HTTPRoutes[key] = httpRouteContext
	}

	return nil
}
