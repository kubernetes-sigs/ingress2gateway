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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	Client client.Client
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
	ReadResourcesFromCluster(ctx context.Context, customResources interface{}) error

	// ReadResourcesFromFiles reads custom resources associated with
	// the underlying Provider implementation from the files.
	ReadResourcesFromFiles(ctx context.Context, customResources interface{}, filename string) error
}

// The ResourceConverter interface specifies all the implemented Gateway API resource
// conversion functions.
type ResourceConverter interface {

	// ToGateway converts the received IngressResources associated
	// with the Provider into GatewayResources.
	ToGateway(resources IngressResources) (GatewayResources, field.ErrorList)
}

// IngressResources contains all Ingress related objects, and Provider specific
// custom resources.
type IngressResources struct {
	Ingresses       []networkingv1.Ingress
	CustomResources interface{}
}

// GatewayResources contains all Gateway-API objects.
type GatewayResources struct {
	Gateways   map[GatewayKey]gatewayv1beta1.Gateway
	HTTPRoutes map[HTTPRouteKey]gatewayv1beta1.HTTPRoute
}

// GatewayKey is a unique identifier for a gateway object.
// Constructed by namespace:name.
type GatewayKey string

// GatewayToGatewayKey assembles the GatewayKey out of a Gateway.
func GatewayToGatewayKey(g gatewayv1beta1.Gateway) GatewayKey {
	return GatewayKey(g.Namespace + ":" + g.Name)
}

// HTTPRouteKey is a unique identifier for an HTTPRoute object.
// Constructed by namespace:name.
type HTTPRouteKey string

// HTTPRouteToHTTPRouteKey assembles the HTTPRouteKey out of an HTTPRoute.
func HTTPRouteToHTTPRouteKey(r gatewayv1beta1.HTTPRoute) HTTPRouteKey {
	return HTTPRouteKey(r.Namespace + ":" + r.Name)
}

// FeatureParser is a function that reads the IngressResources, and applies
// the appropriate modifications to the GatewayResources.
//
// Different FeatureParsers will run in undetermined order. The function must
// modify / create only the required fields of the gateway resources and nothing else.
type FeatureParser func(IngressResources, *GatewayResources) field.ErrorList
