/*
Copyright 2025 The Kubernetes Authors.

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

package annotations

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// SSLRedirectFeature converts SSL redirect annotations to Gateway API RequestRedirect filters.
// Both nginx.org/redirect-to-https and ingress.kubernetes.io/ssl-redirect function identically.
func SSLRedirectFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			modernRedirect, modernExists := rule.Ingress.Annotations[nginxRedirectToHTTPSAnnotation]
			legacyRedirect, legacyExists := rule.Ingress.Annotations[legacySSLRedirectAnnotation]

			// Check if either SSL redirect annotation is enabled
			//nolint:staticcheck
			if !((modernExists && modernRedirect == "true") || (legacyExists && legacyRedirect == "true")) {
				continue
			}

			for _, ingressRule := range rule.Ingress.Spec.Rules {
				ensureHTTPSListener(rule.Ingress, ingressRule, ir)

				routeName := common.RouteName(rule.Ingress.Name, ingressRule.Host)
				routeKey := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: routeName}
				httpRouteContext, routeExists := ir.HTTPRoutes[routeKey]
				if !routeExists {
					continue
				}

				// Update parentRefs to specify the HTTP listener for SSL redirect
				httpListenerName := fmt.Sprintf("%s-http", strings.ReplaceAll(ingressRule.Host, ".", "-"))
				for i := range httpRouteContext.HTTPRoute.Spec.ParentRefs {
					httpRouteContext.HTTPRoute.Spec.ParentRefs[i].SectionName = (*gatewayv1.SectionName)(&httpListenerName)
				}

				// Add redirect rule at the beginning to redirect all HTTP traffic to HTTPS
				redirectRule := gatewayv1.HTTPRouteRule{
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
								Scheme:     ptr.To("https"),
								StatusCode: ptr.To(301),
							},
						},
					},
				}
				httpRouteContext.HTTPRoute.Spec.Rules = append([]gatewayv1.HTTPRouteRule{redirectRule}, httpRouteContext.HTTPRoute.Spec.Rules...)

				ir.HTTPRoutes[routeKey] = httpRouteContext
			}
		}
	}

	return errs
}

// ensureHTTPSListener ensures that a Gateway resource has an HTTPS listener configured
// for the specified Ingress rule. If it doesn't, one is created.
func ensureHTTPSListener(ingress networkingv1.Ingress, rule networkingv1.IngressRule, ir *providerir.ProviderIR) {
	gatewayName := NginxIngressClass
	if ingress.Spec.IngressClassName != nil {
		gatewayName = *ingress.Spec.IngressClassName
	}
	gatewayKey := types.NamespacedName{Namespace: ingress.Namespace, Name: gatewayName}
	gatewayContext, exists := ir.Gateways[gatewayKey]
	if !exists {
		return
	}

	hostname := gatewayv1.Hostname(rule.Host)
	for _, listener := range gatewayContext.Gateway.Spec.Listeners {
		if listener.Protocol == gatewayv1.HTTPSProtocolType && (listener.Hostname == nil || *listener.Hostname == hostname) {
			return
		}
	}

	httpsListener := gatewayv1.Listener{
		Name:     gatewayv1.SectionName(fmt.Sprintf("https-%s", strings.ReplaceAll(rule.Host, ".", "-"))),
		Protocol: gatewayv1.HTTPSProtocolType,
		Port:     443,
		Hostname: &hostname,
		TLS: &gatewayv1.ListenerTLSConfig{
			Mode: ptr.To(gatewayv1.TLSModeTerminate),
			CertificateRefs: []gatewayv1.SecretObjectReference{
				{
					Group: ptr.To(gatewayv1.Group("")),
					Kind:  ptr.To(gatewayv1.Kind("Secret")),
					Name:  gatewayv1.ObjectName(fmt.Sprintf("%s-tls", strings.ReplaceAll(rule.Host, ".", "-"))),
				},
			},
		},
	}
	gatewayContext.Gateway.Spec.Listeners = append(gatewayContext.Gateway.Spec.Listeners, httpsListener)
	ir.Gateways[gatewayKey] = gatewayContext
}
