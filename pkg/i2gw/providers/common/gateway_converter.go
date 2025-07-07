/*
Copyright 2024 The Kubernetes Authors.

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

package common

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ToGatewayResources converts the received intermediate.IR to i2gw.GatewayResource
// without taking into consideration any provider specific logic.
func ToGatewayResources(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources := i2gw.GatewayResources{
		Gateways:           make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes:         make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		GatewayClasses:     ir.GatewayClasses,
		GRPCRoutes:         ir.GRPCRoutes,
		TLSRoutes:          ir.TLSRoutes,
		TCPRoutes:          ir.TCPRoutes,
		UDPRoutes:          ir.UDPRoutes,
		BackendTLSPolicies: ir.BackendTLSPolicies,
		ReferenceGrants:    ir.ReferenceGrants,
	}
	for key, gatewayContext := range ir.Gateways {
		gatewayResources.Gateways[key] = gatewayContext.Gateway
	}
	for key, httpRouteContext := range ir.HTTPRoutes {
		gatewayResources.HTTPRoutes[key] = httpRouteContext.HTTPRoute
	}
	return gatewayResources, nil
}
