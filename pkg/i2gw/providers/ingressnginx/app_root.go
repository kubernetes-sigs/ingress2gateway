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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyAppRootToEmitterIR processes the app-root annotation and creates an HTTPRoute
// that redirects requests from "/" to the specified app-root path.
//
// The nginx.ingress.kubernetes.io/app-root annotation defines a redirect from "/"
// to the specified path. For example, app-root="/dashboard" will redirect requests
// to "/" to "/dashboard".
func applyAppRootToEmitterIR(pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) field.ErrorList {
	var errs field.ErrorList

	// Track which routes we've already processed to avoid duplicates
	processedRoutes := make(map[string]bool)

	for key, httpRouteContext := range pIR.HTTPRoutes {
		for _, sources := range httpRouteContext.RuleBackendSources {
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			appRoot := ingress.Annotations[AppRootAnnotation]
			if appRoot == "" {
				continue
			}

			// Validate the app-root path
			if !strings.HasPrefix(appRoot, "/") {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", AppRootAnnotation),
					appRoot,
					"app-root must start with '/'",
				))
				continue
			}

			// Create a unique key for this HTTPRoute to avoid duplicates
			routeKey := fmt.Sprintf("%s/%s", key.Namespace, key.Name)
			if processedRoutes[routeKey] {
				continue
			}
			processedRoutes[routeKey] = true

			// Create the app-root redirect route
			redirectRoute := createAppRootRedirectRoute(httpRouteContext.HTTPRoute, appRoot)

			eIR.HTTPRoutes[types.NamespacedName{
				Namespace: redirectRoute.Namespace,
				Name:      redirectRoute.Name,
			}] = emitterir.HTTPRouteContext{
				HTTPRoute: redirectRoute,
			}

			notify(notifications.InfoNotification,
				fmt.Sprintf("Created app-root redirect route from '/' to '%s'", appRoot),
				ingress)
		}
	}

	return errs
}

// createAppRootRedirectRoute creates an HTTPRoute that redirects requests from "/"
// to the specified app-root path.
// Uses HTTP 302 (temporary redirect) to match nginx-ingress default behavior.
func createAppRootRedirectRoute(baseRoute gatewayv1.HTTPRoute, appRoot string) gatewayv1.HTTPRoute {
	exactMatch := gatewayv1.PathMatchExact
	specCopy := baseRoute.Spec.DeepCopy()

	return gatewayv1.HTTPRoute{
		TypeMeta: baseRoute.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-app-root", baseRoute.Name),
			Namespace: baseRoute.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: specCopy.ParentRefs,
			},
			Hostnames: specCopy.Hostnames,
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &exactMatch,
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
									ReplaceFullPath: ptr.To(appRoot),
								},
								StatusCode: ptr.To(302),
							},
						},
					},
				},
			},
		},
	}
}
