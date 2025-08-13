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

package ingressnginx

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// serverAliasFeature processes the server-alias annotation for ingress-nginx.
// https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#server-alias
func serverAliasFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
	var errs field.ErrorList

	// Keep track of server aliases per gateway
	serverAliasesByGateway := make(map[types.NamespacedName][]string)

	for _, ingress := range ingresses {
		if serverAlias, ok := ingress.Annotations["nginx.ingress.kubernetes.io/server-alias"]; ok && serverAlias != "" {
			aliases := strings.Split(serverAlias, ",")

			// Determine which gateway this ingress belongs to
			// For ingress-nginx, the gateway name is typically the ingress class name
			ingressClass := common.GetIngressClass(ingress)
			if ingressClass == "" {
				ingressClass = "nginx" // default for ingress-nginx
			}

			gatewayKey := types.NamespacedName{Namespace: ingress.Namespace, Name: ingressClass}

			// Collect all unique aliases for this gateway
			existingAliases := serverAliasesByGateway[gatewayKey]
			for _, alias := range aliases {
				alias = strings.TrimSpace(alias)
				// Check if alias already exists for this gateway
				exists := slices.Contains(existingAliases, alias)
				if !exists {
					serverAliasesByGateway[gatewayKey] = append(serverAliasesByGateway[gatewayKey], alias)
				}
			}

			notify(notifications.InfoNotification,
				fmt.Sprintf("parsed server-alias annotation with %d aliases: %s", len(aliases), strings.Join(aliases, ", ")), &ingress)
		}
	}

	// Process each gateway that has server aliases
	for gatewayKey, aliases := range serverAliasesByGateway {
		// Update HTTPRoutes to include the server aliases as additional hostnames
		for routeKey, routeContext := range ir.HTTPRoutes {
			if routeKey.Namespace == gatewayKey.Namespace {
				// Add server aliases as additional hostnames to the HTTPRoute
				for _, alias := range aliases {
					hostname := gatewayv1.Hostname(alias)
					// Check if this hostname already exists
					exists := slices.Contains(routeContext.Spec.Hostnames, hostname)

					if !exists {
						routeContext.Spec.Hostnames = append(routeContext.Spec.Hostnames, hostname)
						notify(notifications.InfoNotification,
							fmt.Sprintf("added server alias hostname to HTTPRoute: %s", alias),
							&routeContext.HTTPRoute)

						// Update the route context in the IR
						ir.HTTPRoutes[routeKey] = routeContext
					}
				}
			}
		}
	}
	return errs
}
