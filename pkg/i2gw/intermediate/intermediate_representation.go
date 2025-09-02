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
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// IR holds specifications of Gateway Objects for supporting Ingress extensions,
// annotations, and proprietary API features not supported as Gateway core
// features. An IR field can be mapped to core Gateway-API fields,
// or provider-specific Gateway extensions.
type IR struct {
	Gateways   map[types.NamespacedName]GatewayContext
	HTTPRoutes map[types.NamespacedName]HTTPRouteContext
	Services   map[types.NamespacedName]ProviderSpecificServiceIR

	GatewayClasses map[types.NamespacedName]gatewayv1.GatewayClass
	TLSRoutes      map[types.NamespacedName]gatewayv1alpha2.TLSRoute
	TCPRoutes      map[types.NamespacedName]gatewayv1alpha2.TCPRoute
	UDPRoutes      map[types.NamespacedName]gatewayv1alpha2.UDPRoute
	GRPCRoutes     map[types.NamespacedName]gatewayv1.GRPCRoute

	BackendTLSPolicies map[types.NamespacedName]gatewayv1alpha3.BackendTLSPolicy
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
	Apisix       *ApisixGatewayIR
	Cilium       *CiliumGatewayIR
	Gce          *GceGatewayIR
	IngressNginx *IngressNginxGatewayIR
	Istio        *IstioGatewayIR
	Kong         *KongGatewayIR
	Nginx        *NginxGatewayIR
	Openapi3     *Openapi3GatewayIR
}

// HTTPRouteContext contains the Gateway-API HTTPRoute object and HTTPRouteIR,
// which has a dedicated field for each provider to specify their extension
// features on HTTPRoutes.
// The IR will contain necessary information to construct the HTTPRoute
// extensions, but not the extensions themselves.
type HTTPRouteContext struct {
	gatewayv1.HTTPRoute
	ProviderSpecificIR ProviderSpecificHTTPRouteIR
}

type ProviderSpecificHTTPRouteIR struct {
	Apisix       *ApisixHTTPRouteIR
	Cilium       *CiliumHTTPRouteIR
	Gce          *GceHTTPRouteIR
	IngressNginx *IngressNginxHTTPRouteIR
	Istio        *IstioHTTPRouteIR
	Kong         *KongHTTPRouteIR
	Nginx        *NginxHTTPRouteIR
	Openapi3     *Openapi3HTTPRouteIR
}

// ServiceIR contains a dedicated field for each provider to specify their
// extension features on Service.
type ProviderSpecificServiceIR struct {
	Apisix       *ApisixServiceIR
	Cilium       *CiliumServiceIR
	Gce          *GceServiceIR
	IngressNginx *IngressNginxServiceIR
	Istio        *IstioServiceIR
	Kong         *KongServiceIR
	Openapi3     *Openapi3ServiceIR
	Nginx        *NginxServiceIR
}
