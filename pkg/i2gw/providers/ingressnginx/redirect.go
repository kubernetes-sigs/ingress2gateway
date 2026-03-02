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
	"strconv"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Ingress NGINX has some quirky behaviors around SSL redirect.
// The formula we follow is that if an ingress has certs configured, and it does not have the
// "nginx.ingress.kubernetes.io/ssl-redirect" annotation set to "false" (or "0", etc), then we
// enable SSL redirect for that path. This is evaluated per-rule to match ingress-nginx's
// per-location redirect semantics.
func addDefaultSSLRedirect(pir *providerir.ProviderIR, eir *emitterir.EmitterIR) field.ErrorList {
	for key, httpRouteContext := range pir.HTTPRoutes {
		eRouteCtx, ok := eir.HTTPRoutes[key]
		if !ok {
			continue
		}

		var redirectRules []gatewayv1.HTTPRouteRule
		var nonRedirectRules []gatewayv1.HTTPRouteRule
		for ruleIdx, sources := range httpRouteContext.RuleBackendSources {
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			if ruleIdx >= len(eRouteCtx.Spec.Rules) {
				continue
			}

			if len(ingress.Spec.TLS) == 0 {
				continue
			}

			enableRedirect := true
			if val, ok := ingress.Annotations[SSLRedirectAnnotation]; ok {
				parsed, err := strconv.ParseBool(val)
				if err != nil {
					return field.ErrorList{field.Invalid(
						field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations"),
						ingress.Annotations,
						fmt.Sprintf("failed to parse redirect configuration: %v", err),
					)}
				}
				enableRedirect = parsed
			}

			if enableRedirect {
				rule := gatewayv1.HTTPRouteRule{
					Matches: eRouteCtx.Spec.Rules[ruleIdx].DeepCopy().Matches,
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
								Scheme:     ptr.To("https"),
								StatusCode: ptr.To(308),
							},
						},
					},
				}
				redirectRules = append(redirectRules, rule)
			} else {
				nonRedirectRules = append(nonRedirectRules, *eRouteCtx.Spec.Rules[ruleIdx].DeepCopy())
			}
		}

		if len(redirectRules) == 0 {
			continue
		}

		redirectRoute := gatewayv1.HTTPRoute{
			TypeMeta: httpRouteContext.HTTPRoute.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-ssl-redirect", httpRouteContext.HTTPRoute.Name),
				Namespace: httpRouteContext.HTTPRoute.Namespace,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				Hostnames: httpRouteContext.HTTPRoute.Spec.DeepCopy().Hostnames,
				Rules:     redirectRules,
			},
		}
		// add parentrefs
		redirectRoute.Spec.ParentRefs = httpRouteContext.HTTPRoute.Spec.DeepCopy().ParentRefs
		// bind to port 80
		for i := range redirectRoute.Spec.ParentRefs {
			redirectRoute.Spec.ParentRefs[i].Port = ptr.To[int32](80)
		}
		eir.HTTPRoutes[types.NamespacedName{
			Namespace: redirectRoute.Namespace,
			Name:      redirectRoute.Name,
		}] = emitterir.HTTPRouteContext{
			HTTPRoute: redirectRoute,
		}

		// bind original route to port 443
		for i := range eRouteCtx.Spec.ParentRefs {
			eRouteCtx.Spec.ParentRefs[i].Port = ptr.To[int32](443)
		}
		eir.HTTPRoutes[key] = eRouteCtx

		// If some rules don't redirect, they still need to be reachable on port 80.
		// Create a passthrough route on port 80 for those paths.
		if len(nonRedirectRules) > 0 {
			httpRoute := gatewayv1.HTTPRoute{
				TypeMeta: httpRouteContext.HTTPRoute.TypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-http", httpRouteContext.HTTPRoute.Name),
					Namespace: httpRouteContext.HTTPRoute.Namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: httpRouteContext.HTTPRoute.Spec.DeepCopy().Hostnames,
					Rules:     nonRedirectRules,
				},
			}
			httpRoute.Spec.ParentRefs = httpRouteContext.HTTPRoute.Spec.DeepCopy().ParentRefs
			for i := range httpRoute.Spec.ParentRefs {
				httpRoute.Spec.ParentRefs[i].Port = ptr.To[int32](80)
			}
			eir.HTTPRoutes[types.NamespacedName{
				Namespace: httpRoute.Namespace,
				Name:      httpRoute.Name,
			}] = emitterir.HTTPRouteContext{
				HTTPRoute: httpRoute,
			}
		}
	}
	return nil
}
