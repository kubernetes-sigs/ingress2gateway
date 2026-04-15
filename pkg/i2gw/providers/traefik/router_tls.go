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

package traefik

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// routerTLSFeature translates the traefik.ingress.kubernetes.io/router.tls annotation.
//
// When set to "true" and the Ingress has no TLS block defined, Traefik terminates TLS
// using its default certificate store (e.g. Let's Encrypt via ACME). In Gateway API
// terms the closest equivalent is adding an HTTPS listener on the Gateway.
//
// Because no Kubernetes Secret holds the certificate in this case, a conventional
// placeholder secret name is generated from the hostname: "{hostname-with-dashes}-tls"
// (e.g. "my-app-example-com-tls"). An info notification tells the user to create that
// secret (e.g. via cert-manager) before applying the output.
func routerTLSFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			val, ok := rule.Ingress.Annotations[RouterTLSAnnotation]
			parsed, err := strconv.ParseBool(val)
			if !ok || err != nil || !parsed {
				continue
			}

			// If the Ingress already carries a tls block, the standard converter
			// already emitted an HTTPS listener — nothing extra to do.
			if len(rule.Ingress.Spec.TLS) > 0 {
				continue
			}

			// Find the Gateway that was created for this rule group.
			// common.ToIR() names the Gateway after the ingressClass, not the ingress name.
			gatewayKey := types.NamespacedName{
				Namespace: rule.Ingress.Namespace,
				Name:      rg.IngressClass,
			}
			gw, found := ir.Gateways[gatewayKey]
			if !found {
				errs = append(errs, field.NotFound(
					field.NewPath("Gateway"),
					fmt.Sprintf("%s (from ingress %s/%s)", gatewayKey, rule.Ingress.Namespace, rule.Ingress.Name),
				))
				continue
			}

			// Generate a conventional placeholder secret name from the hostname:
			// "my-app.example.com" → "my-app-example-com-tls"
			// The user must create this secret (e.g. via cert-manager) before applying.
			tlsSecretName := gatewayv1.ObjectName(
				fmt.Sprintf("%s-tls", strings.ReplaceAll(rg.Host, ".", "-")),
			)

			httpsListener := gatewayv1.Listener{
				Name:     gatewayv1.SectionName(httpsListenerName(rg.Host)),
				Hostname: listenerHostname(rg.Host),
				Port:     443,
				Protocol: gatewayv1.HTTPSProtocolType,
				TLS: &gatewayv1.ListenerTLSConfig{
					Mode: ptr.To(gatewayv1.TLSModeTerminate),
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{
							Group: ptr.To(gatewayv1.Group("")),
							Kind:  ptr.To(gatewayv1.Kind("Secret")),
							Name:  tlsSecretName,
						},
					},
				},
			}

			// Avoid adding a duplicate listener.
			alreadyHasHTTPS := false
			for _, l := range gw.Spec.Listeners {
				if l.Name == httpsListener.Name {
					alreadyHasHTTPS = true
					break
				}
			}
			if !alreadyHasHTTPS {
				gw.Spec.Listeners = append(gw.Spec.Listeners, httpsListener)
				ir.Gateways[gatewayKey] = gw

				routeKey := types.NamespacedName{
					Namespace: rule.Ingress.Namespace,
					Name:      common.RouteName(rg.Name, rg.Host),
				}
				httpRoute := ir.HTTPRoutes[routeKey]
				notify(
					notifications.InfoNotification,
					fmt.Sprintf(
						"parsed %q annotation: added HTTPS listener to Gateway %s with placeholder "+
							"certificateRef %q — create this secret (e.g. via cert-manager) before applying",
						RouterTLSAnnotation, gatewayKey, tlsSecretName,
					),
					&httpRoute.HTTPRoute,
				)
			}
		}
	}
	return errs
}

func listenerHostname(host string) *gatewayv1.Hostname {
	if host == "" {
		return nil
	}
	h := gatewayv1.Hostname(host)
	return &h
}
