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

package common

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Emitter is the default emitter that converts intermediate representation (IR)
// to standard Gateway API resources without any implementation-specific customizations.
// This emitter is used when no specific emitter is specified via the --emitter flag.
type Emitter struct{}

var _ i2gw.Emitter = &Emitter{}

// init registers the common emitter as the default emitter in the registry.
// It is registered with an empty string key, making it the default when --emitter flag is not specified.
func init() {
	i2gw.EmitterByName[""] = &Emitter{}
}

// ToGatewayResources converts the received intermediate.IR to i2gw.GatewayResource
// without taking into consideration any provider specific logic.
func (e *Emitter) ToGatewayResources(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
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
		hr := gatewayResources.HTTPRoutes[key]
		for i := range hr.Spec.Rules {
			hr.Spec.Rules[i].BackendRefs = removeBackendRefsDuplicates(hr.Spec.Rules[i].BackendRefs)
		}
		gatewayResources.HTTPRoutes[key] = hr
	}
	return gatewayResources, nil
}
