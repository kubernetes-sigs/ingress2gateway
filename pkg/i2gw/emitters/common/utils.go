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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type uniqueBackendRefsKey struct {
	Name      gatewayv1.ObjectName
	Namespace gatewayv1.Namespace
	Port      gatewayv1.PortNumber
	Group     gatewayv1.Group
	Kind      gatewayv1.Kind
}

// removeBackendRefsDuplicates removes duplicate backendRefs from a list of backendRefs.
func removeBackendRefsDuplicates(backendRefs []gatewayv1.HTTPBackendRef) []gatewayv1.HTTPBackendRef {

	uniqueBackendRefs := map[uniqueBackendRefsKey]*gatewayv1.HTTPBackendRef{}

	for _, backendRef := range backendRefs {
		var k uniqueBackendRefsKey

		group := gatewayv1.Group("")
		kind := gatewayv1.Kind("Service")

		if backendRef.Group != nil && *backendRef.Group != "core" {
			group = *backendRef.Group
		}

		if backendRef.Kind != nil {
			kind = *backendRef.Kind
		}

		k.Name = backendRef.Name
		k.Group = group
		k.Kind = kind

		if backendRef.Port != nil {
			k.Port = *backendRef.Port
		}

		if oldRef, exists := uniqueBackendRefs[k]; exists {
			if oldRef.Weight != nil && backendRef.Weight != nil {
				*oldRef.Weight += *backendRef.Weight
			}
		} else {
			uniqueBackendRefs[k] = backendRef.DeepCopy()
		}
	}
	result := make([]gatewayv1.HTTPBackendRef, 0, len(uniqueBackendRefs))
	for _, backendRef := range uniqueBackendRefs {
		result = append(result, *backendRef)
	}
	return result
}

// ToGatewayResources converts the received provider_intermediate.IR to i2gw.GatewayResource
// without taking into consideration any emitter specific logic.
func ToGatewayResources(ir provider_intermediate.ProviderIR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources := i2gw.GatewayResources{
		Gateways:           make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes:         make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		GatewayClasses:     make(map[types.NamespacedName]gatewayv1.GatewayClass),
		GRPCRoutes:         make(map[types.NamespacedName]gatewayv1.GRPCRoute),
		TLSRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		UDPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.UDPRoute),
		BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy),
		ReferenceGrants:    make(map[types.NamespacedName]gatewayv1beta1.ReferenceGrant),
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
	for key, val := range ir.GatewayClasses {
		gatewayResources.GatewayClasses[key] = val
	}
	for key, val := range ir.GRPCRoutes {
		gatewayResources.GRPCRoutes[key] = val
	}
	for key, val := range ir.TLSRoutes {
		gatewayResources.TLSRoutes[key] = val
	}
	for key, val := range ir.TCPRoutes {
		gatewayResources.TCPRoutes[key] = val
	}
	for key, val := range ir.UDPRoutes {
		gatewayResources.UDPRoutes[key] = val
	}
	for key, val := range ir.BackendTLSPolicies {
		gatewayResources.BackendTLSPolicies[key] = val
	}
	for key, val := range ir.ReferenceGrants {
		gatewayResources.ReferenceGrants[key] = val
	}
	return gatewayResources, nil
}
