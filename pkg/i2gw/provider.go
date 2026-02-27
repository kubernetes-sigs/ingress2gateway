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
	"io"
	"sync"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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
	Client                client.Client
	Namespace             string
	ProviderSpecificFlags map[string]map[string]string
	Report                *notifications.Report
}

// The Provider interface specifies the required functionality which needs to be
// implemented by every concrete Ingress/Gateway-API provider, in order for it to
// be used.
type Provider interface {
	CustomResourceReader
	ResourcesToIRConverter
}

type CustomResourceReader interface {

	// ReadResourcesFromCluster reads custom resources associated with
	// the underlying Provider implementation from the kubernetes cluster.
	ReadResourcesFromCluster(ctx context.Context) error

	// ReadResourcesFromFile reads custom resources associated with
	// the underlying Provider implementation from the file.
	ReadResourcesFromFile(ctx context.Context, reader io.Reader) error
}

// The ResourcesToIRConverter interface specifies conversion functions from Ingress
// and extensions into IR.
type ResourcesToIRConverter interface {
	// ToIR converts stored API entities associated with the Provider into IR.
	ToIR() (emitterir.EmitterIR, field.ErrorList)
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

// FeatureParser is a function that reads the Ingresses, and applies
// the appropriate modifications to the providerir.ProviderIR.
//
// Different FeatureParsers will run in undetermined order. The function must
// modify / create only the required fields of the IR and nothing else.
type FeatureParser func(notifications.NotifyFunc, []networkingv1.Ingress, map[types.NamespacedName]map[string]int32, *providerir.ProviderIR) field.ErrorList

var providerSpecificFlagDefinitions = providerSpecificFlags{
	flags: make(map[ProviderName]map[string]ProviderSpecificFlag),
	mu:    sync.RWMutex{},
}

type providerSpecificFlags struct {
	flags map[ProviderName]map[string]ProviderSpecificFlag
	mu    sync.RWMutex // thread-safe, so provider-specific flags can be registered concurrently.
}

type ProviderSpecificFlag struct {
	Name         string
	Description  string
	DefaultValue string
}

func (f *providerSpecificFlags) add(provider ProviderName, flag ProviderSpecificFlag) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.flags[provider] == nil {
		f.flags[provider] = map[string]ProviderSpecificFlag{}
	}
	f.flags[provider][flag.Name] = flag
}

func (f *providerSpecificFlags) all() map[ProviderName]map[string]ProviderSpecificFlag {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.flags
}

// RegisterProviderSpecificFlag registers a provider-specific flag.
// Each provider-specific flag is exposed to the user as an optional command-line flag --<provider>-<flag>.
// If the flag is not provided, it is up to the provider to decide to use the default value or raise an error.
// The provider can read the values of provider-specific flags input by the user from the ProviderConf.
// RegisterProviderSpecificFlag is thread-safe.
func RegisterProviderSpecificFlag(provider ProviderName, flag ProviderSpecificFlag) {
	providerSpecificFlagDefinitions.add(provider, flag)
}

// GetProviderSpecificFlagDefinitions returns the provider specific confs registered by the providers.
func GetProviderSpecificFlagDefinitions() map[ProviderName]map[string]ProviderSpecificFlag {
	return providerSpecificFlagDefinitions.all()
}
