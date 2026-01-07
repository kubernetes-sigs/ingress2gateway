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

package emitterir

import (
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"

	"k8s.io/apimachinery/pkg/types"
)

func ToEmitterIR(pIR providerir.ProviderIR) EmitterIR {
	eIR := EmitterIR{
		Gateways:           make(map[types.NamespacedName]GatewayContext),
		HTTPRoutes:         make(map[types.NamespacedName]HTTPRouteContext),
		GatewayClasses:     make(map[types.NamespacedName]GatewayClassContext),
		TLSRoutes:          make(map[types.NamespacedName]TLSRouteContext),
		TCPRoutes:          make(map[types.NamespacedName]TCPRouteContext),
		UDPRoutes:          make(map[types.NamespacedName]UDPRouteContext),
		GRPCRoutes:         make(map[types.NamespacedName]GRPCRouteContext),
		BackendTLSPolicies: make(map[types.NamespacedName]BackendTLSPolicyContext),
		ReferenceGrants:    make(map[types.NamespacedName]ReferenceGrantContext),
	}

	for k, v := range pIR.Gateways {
		eIR.Gateways[k] = GatewayContext{Gateway: v.Gateway}
	}
	for k, v := range pIR.HTTPRoutes {
		eIR.HTTPRoutes[k] = HTTPRouteContext{HTTPRoute: v.HTTPRoute}
	}
	for k, v := range pIR.GatewayClasses {
		eIR.GatewayClasses[k] = GatewayClassContext{GatewayClass: v}
	}
	for k, v := range pIR.TLSRoutes {
		eIR.TLSRoutes[k] = TLSRouteContext{TLSRoute: v}
	}
	for k, v := range pIR.TCPRoutes {
		eIR.TCPRoutes[k] = TCPRouteContext{TCPRoute: v}
	}
	for k, v := range pIR.UDPRoutes {
		eIR.UDPRoutes[k] = UDPRouteContext{UDPRoute: v}
	}
	for k, v := range pIR.GRPCRoutes {
		eIR.GRPCRoutes[k] = GRPCRouteContext{GRPCRoute: v}
	}
	for k, v := range pIR.BackendTLSPolicies {
		eIR.BackendTLSPolicies[k] = BackendTLSPolicyContext{BackendTLSPolicy: v}
	}
	for k, v := range pIR.ReferenceGrants {
		eIR.ReferenceGrants[k] = ReferenceGrantContext{ReferenceGrant: v}
	}

	return eIR
}
