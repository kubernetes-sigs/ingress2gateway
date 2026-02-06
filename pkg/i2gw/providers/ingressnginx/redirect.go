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
// enable SSL redirect for that host.
func addDefaultSSLRedirect(pir *providerir.ProviderIR, eir *emitterir.EmitterIR) field.ErrorList {
	for key, httpRouteContext := range pir.HTTPRoutes {
		hasSecrets := false
		enableRedirect := true
		forceRedirect := false

		for _, sources := range httpRouteContext.RuleBackendSources {
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			// Check if the ingress has TLS secrets.
			if len(ingress.Spec.TLS) > 0 {
				hasSecrets = true
			}

			// Check the force-ssl-redirect annotation.
			if val, ok := ingress.Annotations[ForceSSLRedirectAnnotation]; ok {
				parsed, err := strconv.ParseBool(val)
				if err != nil {
					return field.ErrorList{field.Invalid(
						field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations"),
						ingress.Annotations,
						fmt.Sprintf("failed to parse force-ssl-redirect configuration: %v", err),
					)}
				}
				forceRedirect = parsed
			}

			// Check the ssl-redirect annotation.
			if val, ok := ingress.Annotations[SSLRedirectAnnotation]; ok {
				parsed, err := strconv.ParseBool(val)
				if err != nil {
					return field.ErrorList{field.Invalid(
						field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations"),
						ingress.Annotations,
						fmt.Sprintf("failed to parse ssl-redirect configuration: %v", err),
					)}
				}
				enableRedirect = parsed
			}
		}

		if (!hasSecrets && !forceRedirect) || !enableRedirect {
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
				Rules: []gatewayv1.HTTPRouteRule{
					{
						Filters: []gatewayv1.HTTPRouteFilter{
							{
								Type: gatewayv1.HTTPRouteFilterRequestRedirect,
								RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
									Scheme:     ptr.To("https"),
									StatusCode: ptr.To(308),
								},
							},
						},
					},
				},
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
		// bind this to port 443
		eHTTPRouteContext := eir.HTTPRoutes[key]
		for i := range eHTTPRouteContext.Spec.ParentRefs {
			eHTTPRouteContext.Spec.ParentRefs[i].Port = ptr.To[int32](443)
		}
		eir.HTTPRoutes[key] = eHTTPRouteContext
	}
	return nil
}

func addWWWRedirect(pir *providerir.ProviderIR, eir *emitterir.EmitterIR) field.ErrorList {
	for _, httpRouteContext := range pir.HTTPRoutes {
		wwwRedirect := false

		for _, sources := range httpRouteContext.RuleBackendSources {
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			if val, ok := ingress.Annotations[FromToWWWRedirectAnnotation]; ok {
				parsed, err := strconv.ParseBool(val)
				if err != nil {
					return field.ErrorList{field.Invalid(
						field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations"),
						ingress.Annotations,
						fmt.Sprintf("failed to parse from-to-www-redirect configuration: %v", err),
					)}
				}
				wwwRedirect = parsed
			}
		}

		if !wwwRedirect {
			continue
		}

		for _, hostname := range httpRouteContext.HTTPRoute.Spec.Hostnames {
			h := string(hostname)
			if len(h) >= 4 && h[:4] == "www." {
				continue
			}

			redirectRoute := gatewayv1.HTTPRoute{
				TypeMeta: httpRouteContext.HTTPRoute.TypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-www-redirect", httpRouteContext.HTTPRoute.Name),
					Namespace: httpRouteContext.HTTPRoute.Namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: []gatewayv1.Hostname{hostname},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterRequestRedirect,
									RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
										Hostname:   ptr.To(gatewayv1.PreciseHostname("www." + h)),
										StatusCode: ptr.To(308),
									},
								},
							},
						},
					},
				},
			}
			redirectRoute.Spec.ParentRefs = httpRouteContext.HTTPRoute.Spec.DeepCopy().ParentRefs

			eir.HTTPRoutes[types.NamespacedName{
				Namespace: redirectRoute.Namespace,
				Name:      redirectRoute.Name,
			}] = emitterir.HTTPRouteContext{
				HTTPRoute: redirectRoute,
			}
		}
	}
	return nil
}
