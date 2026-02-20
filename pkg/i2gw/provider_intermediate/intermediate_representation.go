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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ProviderIR holds specifications of Gateway Objects for supporting Ingress extensions,
// annotations, and proprietary API features not supported as Gateway core
// features. An ProviderIR field can be mapped to core Gateway-API fields,
// or provider-specific Gateway extensions.
type ProviderIR struct {
	Gateways   map[types.NamespacedName]GatewayContext
	HTTPRoutes map[types.NamespacedName]HTTPRouteContext
	Services   map[types.NamespacedName]ServiceContext

	GatewayClasses map[types.NamespacedName]gatewayv1.GatewayClass
	TLSRoutes      map[types.NamespacedName]gatewayv1alpha2.TLSRoute
	TCPRoutes      map[types.NamespacedName]gatewayv1alpha2.TCPRoute
	UDPRoutes      map[types.NamespacedName]gatewayv1alpha2.UDPRoute
	GRPCRoutes     map[types.NamespacedName]gatewayv1.GRPCRoute

	BackendTLSPolicies map[types.NamespacedName]gatewayv1.BackendTLSPolicy
	ReferenceGrants    map[types.NamespacedName]gatewayv1beta1.ReferenceGrant
}

// GatewayContext contains the Gateway-API Gateway object and GatewayIR, which
// has a dedicated field for each provider to specify their extension features
// on Gateways.
// The IR will contain necessary information to construct the Gateway
// extensions, but not the extensions themselves.
type GatewayContext struct {
	gatewayv1.Gateway
	ProviderSpecificIR ProviderSpecificGatewayIR
}

type ProviderSpecificGatewayIR struct {
	Gce *gce.GatewayIR
}

// HTTPRouteContext contains the Gateway-API HTTPRoute object and HTTPRouteIR,
// which has a dedicated field for each provider to specify their extension
// features on HTTPRoutes.
// The IR will contain necessary information to construct the HTTPRoute
// extensions, but not the extensions themselves.
type HTTPRouteContext struct {
	gatewayv1.HTTPRoute
	ProviderSpecificIR ProviderSpecificHTTPRouteIR

	// RuleBackendSources[i][j] is the source of the jth backend in the ith element of HTTPRoute.Spec.Rules.
	RuleBackendSources [][]BackendSource
}

type ProviderSpecificHTTPRouteIR struct {
	Gce *gce.HTTPRouteIR
}

// ServiceIR contains a dedicated field for each provider to specify their
// extension features on Service.
type ProviderSpecificServiceIR struct {
	Gce *gce.ServiceIR
}

// ServiceContext contains the Gateway-API Service object and ServiceIR, which
// has a dedicated field for each provider to specify their extension features
// on Service.
type ServiceContext struct {
	ProviderSpecificIR ProviderSpecificServiceIR
	SessionAffinity    *SessionAffinityConfig
}

type SessionAffinityConfig struct {
	AffinityType string
	CookieTTLSec *int64
}

// BackendSource tracks the source Ingress resource that contributed
// a specific BackendRef to an HTTPRoute rule.
type BackendSource struct {
	// Source Ingress that contributed this backend
	Ingress *networkingv1.Ingress

	// Exactly one of Path or DefaultBackend must be non-nil.
	// Path points to the specific HTTPIngressPath that contributed this backend.
	Path *networkingv1.HTTPIngressPath

	// DefaultBackend points to the Ingress's spec.defaultBackend that contributed this backend.
	DefaultBackend *networkingv1.IngressBackend
}
