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

package provider_intermediate

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/apimachinery/pkg/types"
)

func ToEmitterIR(pIR ProviderIR) emitter_intermediate.EmitterIR {
	eIR := emitter_intermediate.EmitterIR{
		Gateways:           make(map[types.NamespacedName]emitter_intermediate.GatewayContext),
		HTTPRoutes:         make(map[types.NamespacedName]emitter_intermediate.HTTPRouteContext),
		GatewayClasses:     make(map[types.NamespacedName]emitter_intermediate.GatewayClassContext),
		TLSRoutes:          make(map[types.NamespacedName]emitter_intermediate.TLSRouteContext),
		TCPRoutes:          make(map[types.NamespacedName]emitter_intermediate.TCPRouteContext),
		UDPRoutes:          make(map[types.NamespacedName]emitter_intermediate.UDPRouteContext),
		GRPCRoutes:         make(map[types.NamespacedName]emitter_intermediate.GRPCRouteContext),
		BackendTLSPolicies: make(map[types.NamespacedName]emitter_intermediate.BackendTLSPolicyContext),
		ReferenceGrants:    make(map[types.NamespacedName]emitter_intermediate.ReferenceGrantContext),
	}

	for k, v := range pIR.Gateways {
		eIR.Gateways[k] = emitter_intermediate.GatewayContext{Gateway: v.Gateway}
	}
	for k, v := range pIR.HTTPRoutes {
		eIR.HTTPRoutes[k] = emitter_intermediate.HTTPRouteContext{HTTPRoute: v.HTTPRoute}
	}
	for k, v := range pIR.GatewayClasses {
		eIR.GatewayClasses[k] = emitter_intermediate.GatewayClassContext{GatewayClass: v}
	}
	for k, v := range pIR.TLSRoutes {
		eIR.TLSRoutes[k] = emitter_intermediate.TLSRouteContext{TLSRoute: v}
	}
	for k, v := range pIR.TCPRoutes {
		eIR.TCPRoutes[k] = emitter_intermediate.TCPRouteContext{TCPRoute: v}
	}
	for k, v := range pIR.UDPRoutes {
		eIR.UDPRoutes[k] = emitter_intermediate.UDPRouteContext{UDPRoute: v}
	}
	for k, v := range pIR.GRPCRoutes {
		eIR.GRPCRoutes[k] = emitter_intermediate.GRPCRouteContext{GRPCRoute: v}
	}
	for k, v := range pIR.BackendTLSPolicies {
		eIR.BackendTLSPolicies[k] = emitter_intermediate.BackendTLSPolicyContext{BackendTLSPolicy: v}
	}
	for k, v := range pIR.ReferenceGrants {
		eIR.ReferenceGrants[k] = emitter_intermediate.ReferenceGrantContext{ReferenceGrant: v}
	}

	return eIR
}
