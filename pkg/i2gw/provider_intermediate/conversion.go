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

package providerir

import (
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	"k8s.io/apimachinery/pkg/types"
)

func ToEmitterIR(pIR ProviderIR) emitterir.EmitterIR {
	eIR := emitterir.EmitterIR{
		Gateways:           make(map[types.NamespacedName]emitterir.GatewayContext),
		HTTPRoutes:         make(map[types.NamespacedName]emitterir.HTTPRouteContext),
		GatewayClasses:     make(map[types.NamespacedName]emitterir.GatewayClassContext),
		TLSRoutes:          make(map[types.NamespacedName]emitterir.TLSRouteContext),
		TCPRoutes:          make(map[types.NamespacedName]emitterir.TCPRouteContext),
		UDPRoutes:          make(map[types.NamespacedName]emitterir.UDPRouteContext),
		GRPCRoutes:         make(map[types.NamespacedName]emitterir.GRPCRouteContext),
		BackendTLSPolicies: make(map[types.NamespacedName]emitterir.BackendTLSPolicyContext),
		ReferenceGrants:    make(map[types.NamespacedName]emitterir.ReferenceGrantContext),
	}

	for k, v := range pIR.Gateways {
		eIR.Gateways[k] = emitterir.GatewayContext{Gateway: v.Gateway}
	}
	for k, v := range pIR.HTTPRoutes {
		eIR.HTTPRoutes[k] = emitterir.HTTPRouteContext{HTTPRoute: v.HTTPRoute}
	}
	for k, v := range pIR.GatewayClasses {
		eIR.GatewayClasses[k] = emitterir.GatewayClassContext{GatewayClass: v}
	}
	for k, v := range pIR.TLSRoutes {
		eIR.TLSRoutes[k] = emitterir.TLSRouteContext{TLSRoute: v}
	}
	for k, v := range pIR.TCPRoutes {
		eIR.TCPRoutes[k] = emitterir.TCPRouteContext{TCPRoute: v}
	}
	for k, v := range pIR.UDPRoutes {
		eIR.UDPRoutes[k] = emitterir.UDPRouteContext{UDPRoute: v}
	}
	for k, v := range pIR.GRPCRoutes {
		eIR.GRPCRoutes[k] = emitterir.GRPCRouteContext{GRPCRoute: v}
	}
	for k, v := range pIR.BackendTLSPolicies {
		eIR.BackendTLSPolicies[k] = emitterir.BackendTLSPolicyContext{BackendTLSPolicy: v}
	}
	for k, v := range pIR.ReferenceGrants {
		eIR.ReferenceGrants[k] = emitterir.ReferenceGrantContext{ReferenceGrant: v}
	}

	eIR.GceServices = make(map[types.NamespacedName]gce.ServiceIR)
	eIR.Services = make(map[types.NamespacedName]emitterir.ServiceContext)
	for k, v := range pIR.Services {
		if v.ProviderSpecificIR.Gce != nil {
			eIR.GceServices[k] = *v.ProviderSpecificIR.Gce
		}
		if v.SessionAffinity != nil {
			eIR.Services[k] = emitterir.ServiceContext{
				SessionAffinity: &emitterir.SessionAffinityConfig{
					AffinityType: v.SessionAffinity.AffinityType,
					CookieTTLSec: v.SessionAffinity.CookieTTLSec,
				},
			}
		}
	}

	return eIR
}
