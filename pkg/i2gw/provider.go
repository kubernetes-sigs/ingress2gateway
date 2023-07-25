/*
Copyright 2022 The Kubernetes Authors.

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
type ProviderConf struct{}

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

	// ConvertHTTPRoutes converts the received ingresses and custom resources
	// associated with the Provider into HTTPRoutes and Gateways.
	ConvertHTTPRoutes(ingresses []networkingv1.Ingress, customResources interface{}) ([]gatewayv1beta1.HTTPRoute, []gatewayv1beta1.Gateway, field.ErrorList)
}
