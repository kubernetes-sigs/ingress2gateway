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

package intermediate

import (
	"fmt"
	"maps"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// MergeIRs accepts multiple IRs and creates a unique IR struct built
// as follows:
//   - GatewayClasses, Routes, and ReferenceGrants are grouped into the same maps
//   - Gateways may have the same NamespaceName even if they come from different
//     ingresses, as they have a their GatewayClass' name as name. For this reason,
//     if there are mutiple gateways named the same, their listeners are merged into
//     a unique Gateway.
//
// This behavior is likely to change after https://github.com/kubernetes-sigs/gateway-api/pull/1863 takes place.
func MergeIRs(irs ...IR) (IR, field.ErrorList) {
	mergedIRs := IR{
		Gateways:           make(map[types.NamespacedName]GatewayContext),
		GatewayClasses:     make(map[types.NamespacedName]gatewayv1.GatewayClass),
		HTTPRoutes:         make(map[types.NamespacedName]HTTPRouteContext),
		Services:           make(map[types.NamespacedName]ProviderSpecificServiceIR),
		GRPCRoutes:         make(map[types.NamespacedName]gatewayv1.GRPCRoute),
		TLSRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		UDPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.UDPRoute),
		BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1alpha3.BackendTLSPolicy),
		ReferenceGrants:    make(map[types.NamespacedName]gatewayv1beta1.ReferenceGrant),
	}
	var errs field.ErrorList
	mergedIRs.Gateways, errs = mergeGatewayContexts(irs)
	if len(errs) > 0 {
		return IR{}, errs
	}
	// TODO(issue #189): Perform merge on HTTPRoute and Service like Gateway.
	for _, gr := range irs {
		maps.Copy(mergedIRs.GatewayClasses, gr.GatewayClasses)
		maps.Copy(mergedIRs.HTTPRoutes, gr.HTTPRoutes)
		maps.Copy(mergedIRs.Services, gr.Services)
		maps.Copy(mergedIRs.GRPCRoutes, gr.GRPCRoutes)
		maps.Copy(mergedIRs.TLSRoutes, gr.TLSRoutes)
		maps.Copy(mergedIRs.TCPRoutes, gr.TCPRoutes)
		maps.Copy(mergedIRs.UDPRoutes, gr.UDPRoutes)
		maps.Copy(mergedIRs.BackendTLSPolicies, gr.BackendTLSPolicies)
		maps.Copy(mergedIRs.ReferenceGrants, gr.ReferenceGrants)
	}
	return mergedIRs, errs
}

func mergeGatewayContexts(irs []IR) (map[types.NamespacedName]GatewayContext, field.ErrorList) {
	newGatewayContexts := make(map[types.NamespacedName]GatewayContext)
	errs := field.ErrorList{}

	for _, currentIR := range irs {
		for _, g := range currentIR.Gateways {
			nn := types.NamespacedName{Namespace: g.Gateway.Namespace, Name: g.Gateway.Name}
			if existingGatewayContext, ok := newGatewayContexts[nn]; ok {
				g.Gateway.Spec.Listeners = append(g.Gateway.Spec.Listeners, existingGatewayContext.Gateway.Spec.Listeners...)
				g.Gateway.Spec.Addresses = append(g.Gateway.Spec.Addresses, existingGatewayContext.Gateway.Spec.Addresses...)
				g.ProviderSpecificIR = mergedGatewayIR(g.ProviderSpecificIR, existingGatewayContext.ProviderSpecificIR)
			}
			newGatewayContexts[nn] = GatewayContext{Gateway: g.Gateway}
			// 64 is the maximum number of listeners a Gateway can have
			if len(g.Spec.Listeners) > 64 {
				fieldPath := field.NewPath(fmt.Sprintf("%s/%s", nn.Namespace, nn.Name)).Child("spec").Child("listeners")
				errs = append(errs, field.Invalid(fieldPath, g, "error while merging gateway listeners: a gateway cannot have more than 64 listeners"))
			}
			// 16 is the maximum number of addresses a Gateway can have
			if len(g.Spec.Addresses) > 16 {
				fieldPath := field.NewPath(fmt.Sprintf("%s/%s", nn.Namespace, nn.Name)).Child("spec").Child("addresses")
				errs = append(errs, field.Invalid(fieldPath, g, "error while merging gateway listeners: a gateway cannot have more than 16 addresses"))
			}
		}
	}
	return newGatewayContexts, errs
}

func mergedGatewayIR(current, existing ProviderSpecificGatewayIR) ProviderSpecificGatewayIR {
	var mergedGatewayIR ProviderSpecificGatewayIR
	// TODO(issue #190): Find a different way to merge GatewayIR, instead of
	// delegating them to each provider.
	mergedGatewayIR.Gce = mergeGceGatewayIR(current.Gce, existing.Gce)
	return mergedGatewayIR
}
