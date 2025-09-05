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

package crds

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	nginxv1 "github.com/nginx/kubernetes-ingress/pkg/apis/configuration/v1"
)

// ToGatewayIR converts nginx VirtualServer, VirtualServerRoute, and TransportServer CRDs to Gateway API resources
// This function creates one shared Gateway per namespace that handles both Layer 7 and Layer 4 traffic
func ToGatewayIR(
	virtualServers []nginxv1.VirtualServer,
	_ []nginxv1.VirtualServerRoute,
	transportServers []nginxv1.TransportServer,
	globalConfiguration *nginxv1.GlobalConfiguration) (
	partial intermediate.IR,
	notificationList []notifications.Notification,
	errs field.ErrorList,
) {
	notificationList = make([]notifications.Notification, 0)

	var validVirtualServers []nginxv1.VirtualServer
	for _, vs := range virtualServers {
		if vs.Spec.Host == "" {
			addNotification(&notificationList, notifications.WarningNotification,
				"VirtualServer has no host specified, skipping", &vs)
			continue
		}
		validVirtualServers = append(validVirtualServers, vs)
	}

	// Check if we have any resources to process
	if len(validVirtualServers) == 0 && len(transportServers) == 0 {
		return intermediate.IR{}, notificationList, errs
	}

	// Group resources by namespace
	namespaceVSMap := make(map[string][]nginxv1.VirtualServer)
	for _, vs := range validVirtualServers {
		namespaceVSMap[vs.Namespace] = append(namespaceVSMap[vs.Namespace], vs)
	}

	namespaceTSMap := make(map[string][]nginxv1.TransportServer)
	for _, ts := range transportServers {
		namespaceTSMap[ts.Namespace] = append(namespaceTSMap[ts.Namespace], ts)
	}

	// Initialize result maps
	gatewayMap := make(map[types.NamespacedName]intermediate.GatewayContext)
	httpRouteMap := make(map[types.NamespacedName]intermediate.HTTPRouteContext)
	backendTLSPoliciesMap := make(map[types.NamespacedName]gatewayv1alpha3.BackendTLSPolicy)
	grpcRouteMap := make(map[types.NamespacedName]gatewayv1.GRPCRoute)
	tcpRouteMap := make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute)
	tlsRouteMap := make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute)
	udpRouteMap := make(map[types.NamespacedName]gatewayv1alpha2.UDPRoute)

	// Build a listener map
	listenerMap := make(map[string]gatewayv1.Listener)
	if globalConfiguration != nil {
		for _, l := range globalConfiguration.Spec.Listeners {
			listenerMap[l.Name] = gatewayv1.Listener{
				Name:     gatewayv1.SectionName(l.Name),
				Port:     gatewayv1.PortNumber(l.Port),
				Protocol: gatewayv1.ProtocolType(l.Protocol),
			}
		}
	}

	// Get all namespaces that have either VirtualServers or TransportServers
	allNamespaces := make(map[string]bool)
	for namespace := range namespaceVSMap {
		allNamespaces[namespace] = true
	}
	for namespace := range namespaceTSMap {
		allNamespaces[namespace] = true
	}

	for namespace := range allNamespaces {
		vsListForNamespace := namespaceVSMap[namespace] // May be empty slice
		tsListForNamespace := namespaceTSMap[namespace] // May be empty slice

		// Create shared gateway for both VirtualServers and TransportServers
		gatewayFactory := NewNamespaceGatewayFactory(namespace, vsListForNamespace, tsListForNamespace, &notificationList, listenerMap)
		gateways, _ := gatewayFactory.CreateNamespaceGateway()

		for gatewayKey, gateway := range gateways {
			gatewayMap[gatewayKey] = gateway
		}

		// TODO: VirtualServer and TransportServer route conversion will be added in subsequent PRs
	}

	return intermediate.IR{
		Gateways:           gatewayMap,
		HTTPRoutes:         httpRouteMap,
		BackendTLSPolicies: backendTLSPoliciesMap,
		GRPCRoutes:         grpcRouteMap,
		TCPRoutes:          tcpRouteMap,
		TLSRoutes:          tlsRouteMap,
		UDPRoutes:          udpRouteMap,
	}, notificationList, errs
}
