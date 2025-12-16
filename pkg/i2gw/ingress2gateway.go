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
	"fmt"
	"sort"

	common_emitter "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/common_emitter"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const GeneratorAnnotationKey = "gateway.networking.k8s.io/generator"

// Version holds the version string (injected by ldflags during build).
// It will be populated by `git describe --tags --always --dirty`.
// Examples: "v0.4.0", "v0.4.0-5-gabcdef", "v0.4.0-5-gabcdef-dirty"
var Version = "dev" // Default value if not built with linker flags

func ToGatewayAPIResources(ctx context.Context, namespace string, inputFile string, providers []string, emitterName string, providerSpecificFlags map[string]map[string]string) ([]GatewayResources, map[string]string, error) {
	var clusterClient client.Client

	if inputFile == "" {
		conf, err := config.GetConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get client config: %w", err)
		}

		cl, err := client.New(conf, client.Options{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create client: %w", err)
		}
		clusterClient = client.NewNamespacedClient(cl, namespace)
	}

	providerByName, err := constructProviders(&ProviderConf{
		Client:                clusterClient,
		Namespace:             namespace,
		ProviderSpecificFlags: providerSpecificFlags,
	}, providers)
	if err != nil {
		return nil, nil, err
	}

	if inputFile != "" {
		if err = readProviderResourcesFromFile(ctx, providerByName, inputFile); err != nil {
			return nil, nil, err
		}
	} else {
		if err = readProviderResourcesFromCluster(ctx, providerByName); err != nil {
			return nil, nil, err
		}
	}

	emitterConf := &EmitterConf{}
	newEmitterFunc, ok := EmitterConstructorByName[EmitterName(emitterName)]
	if !ok {
		return nil, nil, fmt.Errorf("%s is not a supported emitter", emitterName)
	}
	emitter := newEmitterFunc(emitterConf)
	commonEmitter := common_emitter.NewEmitter()

	var (
		gatewayResources []GatewayResources
		errs             field.ErrorList
	)
	for _, provider := range providerByName {
		ir, conversionErrs := provider.ToIR()
		errs = append(errs, conversionErrs...)

		ir, conversionErrs = commonEmitter.Emit(ir)
		errs = append(errs, conversionErrs...)

		providerGatewayResources, conversionErrs := emitter.Emit(ir)
		errs = append(errs, conversionErrs...)
		gatewayResources = append(gatewayResources, providerGatewayResources)
	}
	notificationTablesMap := notifications.NotificationAggr.CreateNotificationTables()
	if len(errs) > 0 {
		return nil, notificationTablesMap, aggregatedErrs(errs)
	}

	return gatewayResources, notificationTablesMap, nil
}

func readProviderResourcesFromFile(ctx context.Context, providerByName map[ProviderName]Provider, inputFile string) error {
	for name, provider := range providerByName {
		if err := provider.ReadResourcesFromFile(ctx, inputFile); err != nil {
			return fmt.Errorf("failed to read %s resources from file: %w", name, err)
		}
	}
	return nil
}

func readProviderResourcesFromCluster(ctx context.Context, providerByName map[ProviderName]Provider) error {
	for name, provider := range providerByName {
		if err := provider.ReadResourcesFromCluster(ctx); err != nil {
			return fmt.Errorf("failed to read %s resources from the cluster: %w", name, err)
		}
	}
	return nil
}

// constructProviders constructs a map of concrete Provider implementations
// by their ProviderName.
func constructProviders(conf *ProviderConf, providers []string) (map[ProviderName]Provider, error) {
	providerByName := make(map[ProviderName]Provider, len(ProviderConstructorByName))

	for _, requestedProvider := range providers {
		requestedProviderName := ProviderName(requestedProvider)
		newProviderFunc, ok := ProviderConstructorByName[requestedProviderName]
		if !ok {
			return nil, fmt.Errorf("%s is not a supported provider", requestedProvider)
		}

		providerByName[requestedProviderName] = newProviderFunc(conf)
	}

	return providerByName, nil
}

func aggregatedErrs(errs field.ErrorList) error {
	errMsg := fmt.Errorf("\n# Encountered %d errors", len(errs))
	for _, err := range errs {
		errMsg = fmt.Errorf("\n%w # %s", errMsg, err.Error())
	}
	return errMsg
}

// GetSupportedProviders returns the names of all providers that are supported now
func GetSupportedProviders() []string {
	supportedProviders := make([]string, 0, len(ProviderConstructorByName))
	for key := range ProviderConstructorByName {
		supportedProviders = append(supportedProviders, string(key))
	}
	// Sort the provider names for consistent output.
	sort.Strings(supportedProviders)
	return supportedProviders
}

// GetSupportedEmitters returns the names of all emitters that are supported now
func GetSupportedEmitters() []string {
	supportedEmitters := make([]string, 0, len(EmitterConstructorByName))
	for key := range EmitterConstructorByName {
		supportedEmitters = append(supportedEmitters, string(key))
	}
	// Sort the emitter names for consistent output.
	sort.Strings(supportedEmitters)
	return supportedEmitters
}

func CastToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	// Convert the Kubernetes object to unstructured.Unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: unstructuredObj}, nil
}
