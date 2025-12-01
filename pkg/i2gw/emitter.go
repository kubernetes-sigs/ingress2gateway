/*
Copyright 2023 The Kubernetes Authors.

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

package i2gw

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type Emitter interface {
	// ToGatewayResources converts stored IR with the Provider into
	// Gateway API resources and extensions
	Emit(provider_intermediate.IR) (GatewayResources, field.ErrorList)
}

// GatewayResources contains all Gateway-API objects and provider Gateway
// extensions.
type GatewayResources struct {
	Gateways       map[types.NamespacedName]gatewayv1.Gateway
	GatewayClasses map[types.NamespacedName]gatewayv1.GatewayClass

	HTTPRoutes map[types.NamespacedName]gatewayv1.HTTPRoute
	GRPCRoutes map[types.NamespacedName]gatewayv1.GRPCRoute
	TLSRoutes  map[types.NamespacedName]gatewayv1alpha2.TLSRoute
	TCPRoutes  map[types.NamespacedName]gatewayv1alpha2.TCPRoute
	UDPRoutes  map[types.NamespacedName]gatewayv1alpha2.UDPRoute

	BackendTLSPolicies map[types.NamespacedName]gatewayv1.BackendTLSPolicy
	ReferenceGrants    map[types.NamespacedName]gatewayv1beta1.ReferenceGrant

	GatewayExtensions []unstructured.Unstructured
}

// ProviderConstructorByName is a map of ProviderConstructor functions by a
// provider name. Different Provider implementations should add their construction
// func at startup.
var EmitterConstructorByName = map[EmitterName]EmitterConstructor{}

type EmitterName string

type EmitterConstructor func(conf *EmitterConf) Emitter

type EmitterConf struct {
}
