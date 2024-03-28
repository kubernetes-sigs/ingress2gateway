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
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ProviderConstructorByName is a map of ProviderConstructor functions by a
// provider name. Different Provider implementations should add their construction
// func at startup.
var ProviderConstructorByName = map[ProviderName]ProviderConstructor{}

// ProviderName is a string alias that stores the concrete Provider name.
type ProviderName string

// ProviderConstructor is a construction function that constructs concrete
// implementations of the Provider interface.
type ProviderConstructor func(conf *ProviderConf) Provider

// ProviderConf contains all the configuration required for every concrete
// Provider implementation.
type ProviderConf struct {
	Client    client.Client
	Namespace string
}

// The Provider interface specifies the required functionality which needs to be
// implemented by every concrete Ingress/Gateway-API provider, in order for it to
// be used.
type Provider interface {
	CustomResourceReader
	ResourceConverter
}

type CustomResourceReader interface {

	// ReadResourcesFromCluster reads custom resources associated with
	// the underlying Provider implementation from the kubernetes cluster.
	ReadResourcesFromCluster(ctx context.Context) error

	// ReadResourcesFromFile reads custom resources associated with
	// the underlying Provider implementation from the file.
	ReadResourcesFromFile(ctx context.Context, filename string) error
}

// The ResourceConverter interface specifies all the implemented Gateway API resource
// conversion functions.
type ResourceConverter interface {

	// ToGatewayAPIResources converts the received InputResources associated
	// with the Provider into GatewayResources.
	ToGatewayAPI(resources InputResources) (GatewayResources, field.ErrorList)
}

// ImplementationSpecificHTTPPathTypeMatchConverter is an option to customize the ingress implementationSpecific
// match type conversion.
type ImplementationSpecificHTTPPathTypeMatchConverter func(*gatewayv1.HTTPPathMatch)

// ProviderImplementationSpecificOptions contains customized implementation-specific fields and functions.
// These will be used by the common package to customize the provider-specific behavior for all the
// implementation-specific fields of the ingress API.
type ProviderImplementationSpecificOptions struct {
	ToImplementationSpecificHTTPPathTypeMatch ImplementationSpecificHTTPPathTypeMatchConverter
}

// InputResources contains all Ingress objects.
type InputResources struct {
	Ingresses []networkingv1.Ingress
}

// GatewayResources contains all Gateway-API objects.
type GatewayResources struct {
	Gateways       map[types.NamespacedName]gatewayv1.Gateway
	GatewayClasses map[types.NamespacedName]gatewayv1.GatewayClass

	HTTPRoutes map[types.NamespacedName]gatewayv1.HTTPRoute
	TLSRoutes  map[types.NamespacedName]gatewayv1alpha2.TLSRoute
	TCPRoutes  map[types.NamespacedName]gatewayv1alpha2.TCPRoute
	UDPRoutes  map[types.NamespacedName]gatewayv1alpha2.UDPRoute

	ReferenceGrants map[types.NamespacedName]gatewayv1beta1.ReferenceGrant
}

// FeatureParser is a function that reads the InputResources, and applies
// the appropriate modifications to the GatewayResources.
//
// Different FeatureParsers will run in undetermined order. The function must
// modify / create only the required fields of the gateway resources and nothing else.
type FeatureParser func(InputResources, *GatewayResources) field.ErrorList
