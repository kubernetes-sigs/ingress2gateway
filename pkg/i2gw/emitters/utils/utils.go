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

package utils

import (
	"reflect"
	"sort"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
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

// ToGatewayResources converts the received emitterir.IR to i2gw.GatewayResource
// without taking into consideration any emitter specific logic.
func ToGatewayResources(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
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
		gatewayResources.GatewayClasses[key] = val.GatewayClass
	}
	for key, val := range ir.GRPCRoutes {
		gatewayResources.GRPCRoutes[key] = val.GRPCRoute
	}
	for key, val := range ir.TLSRoutes {
		gatewayResources.TLSRoutes[key] = val.TLSRoute
	}
	for key, val := range ir.TCPRoutes {
		gatewayResources.TCPRoutes[key] = val.TCPRoute
	}
	for key, val := range ir.UDPRoutes {
		gatewayResources.UDPRoutes[key] = val.UDPRoute
	}
	for key, val := range ir.BackendTLSPolicies {
		gatewayResources.BackendTLSPolicies[key] = val.BackendTLSPolicy
	}
	for key, val := range ir.ReferenceGrants {
		gatewayResources.ReferenceGrants[key] = val.ReferenceGrant
	}
	return gatewayResources, nil
}

// MergeExternalAuth merges per-rule external auth configs into a single
// RouteAllRulesKey entry if all rules have the same configuration.
func MergeExternalAuth(ctx *emitterir.HTTPRouteContext) {
	if len(ctx.ExtAuth) <= 1 {
		return
	}
	if len(ctx.Spec.Rules) != len(ctx.ExtAuth) {
		return
	}

	// Check if all configs are equal
	var firstConfig *emitterir.ExternalAuthConfig
	for _, config := range ctx.ExtAuth {
		if firstConfig == nil {
			firstConfig = config
			continue
		}
		if !externalAuthConfigsEqual(firstConfig, config) {
			return // Configs are not equal, no merge
		}
	}

	// All configs are equal, merge into RouteAllRulesKey
	if firstConfig != nil {
		ctx.ExtAuth = map[int]*emitterir.ExternalAuthConfig{
			emitterir.RouteAllRulesKey: firstConfig,
		}
	}
}

// externalAuthConfigsEqual checks if two ExternalAuthConfig are equal
func externalAuthConfigsEqual(a, b *emitterir.ExternalAuthConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare BackendObjectReference fields
	if !backendObjectReferencesEqual(a.BackendObjectReference, b.BackendObjectReference) {
		return false
	}
	if a.Protocol != b.Protocol {
		return false
	}
	if a.Path != b.Path {
		return false
	}

	sort.Strings(a.AllowedResponseHeaders)
	sort.Strings(b.AllowedResponseHeaders)
	return reflect.DeepEqual(a.AllowedResponseHeaders, b.AllowedResponseHeaders)
}

// backendObjectReferencesEqual checks if two BackendObjectReference are equal
func backendObjectReferencesEqual(a, b gatewayv1.BackendObjectReference) bool {
	if ptr.Deref(a.Group, gatewayv1.Group("")) != ptr.Deref(b.Group, gatewayv1.Group("")) {
		return false
	}
	if ptr.Deref(a.Kind, gatewayv1.Kind("Service")) != ptr.Deref(b.Kind, gatewayv1.Kind("Service")) {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if ptr.Deref(a.Namespace, gatewayv1.Namespace("default")) != ptr.Deref(b.Namespace, gatewayv1.Namespace("default")) {
		return false
	}
	if ptr.Deref(a.Port, gatewayv1.PortNumber(80)) != ptr.Deref(b.Port, gatewayv1.PortNumber(80)) {
		return false
	}
	return true
}
