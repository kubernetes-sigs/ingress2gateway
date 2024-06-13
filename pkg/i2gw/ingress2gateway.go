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
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func GetResources(ctx context.Context, namespace string, inputFile string, allresources bool, providers []string, providerSpecificFlags map[string]map[string]string) ([]client.Object, error) {
	var clusterClient client.Client

	if inputFile == "" {
		conf, err := config.GetConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get client config: %w", err)
		}

		cl, err := client.New(conf, client.Options{})
		if err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		}
		clusterClient = client.NewNamespacedClient(cl, namespace)
	}

	providerByName, err := constructProviders(&ProviderConf{
		Client:                clusterClient,
		Namespace:             namespace,
		ProviderSpecificFlags: providerSpecificFlags,
	}, providers)
	if err != nil {
		return nil, err
	}

	var genericResources []client.Object
	if inputFile != "" {
		if err = readProviderResourcesFromFile(ctx, providerByName, inputFile); err != nil {
			return nil, err
		}
		if allresources {
			if genericResources, err = readGenericResourcesFromFile(inputFile, namespace, providerByName); err != nil {
				return nil, err
			}
		}
	} else {
		if err = readProviderResourcesFromCluster(ctx, providerByName); err != nil {
			return nil, err
		}
	}

	var (
		gatewayResources []GatewayResources
		errs             field.ErrorList
	)
	for _, provider := range providerByName {
		providerGatewayResources, conversionErrs := provider.ToGatewayAPI()
		errs = append(errs, conversionErrs...)
		gatewayResources = append(gatewayResources, providerGatewayResources)
	}
	if len(errs) > 0 {
		return nil, aggregatedErrs(errs)
	}

	return append(gatewayResourcesToObjects(gatewayResources), genericResources...), nil
}

func gatewayResourcesToObjects(gatewayResources []GatewayResources) []client.Object {
	var objects []client.Object
	for _, gr := range gatewayResources {
		for _, gateway := range gr.Gateways {
			gateway := gateway
			objects = append(objects, &gateway)
		}
		for _, gatewayClass := range gr.GatewayClasses {
			gatewayClass := gatewayClass
			objects = append(objects, &gatewayClass)
		}
		for _, httpRoute := range gr.HTTPRoutes {
			httpRoute := httpRoute
			objects = append(objects, &httpRoute)
		}
		for _, tlsRoute := range gr.TLSRoutes {
			tlsRoute := tlsRoute
			objects = append(objects, &tlsRoute)
		}
		for _, tcpRoute := range gr.TCPRoutes {
			tcpRoute := tcpRoute
			objects = append(objects, &tcpRoute)
		}
		for _, udpRoute := range gr.UDPRoutes {
			udpRoute := udpRoute
			objects = append(objects, &udpRoute)
		}
		for _, referenceGrant := range gr.ReferenceGrants {
			referenceGrant := referenceGrant
			objects = append(objects, &referenceGrant)
		}
	}
	return objects
}

func readProviderResourcesFromFile(ctx context.Context, providerByName map[ProviderName]Provider, inputFile string) error {
	for name, provider := range providerByName {
		if err := provider.ReadResourcesFromFile(ctx, inputFile); err != nil {
			return fmt.Errorf("failed to read %s resources from file: %w", name, err)
		}
	}
	return nil
}

func readGenericResourcesFromFile(inputFile, namespace string, providerByName map[ProviderName]Provider) ([]client.Object, error) {
	objects := make([]client.Object, 0)
	stream, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", inputFile, err)
	}

	unstructuredObjects, err := ExtractObjectsFromReader(bytes.NewReader(stream), namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	for _, p := range providerByName {
		for _, crd := range p.GetCRDs() {
			FilteredResources[crd] = struct{}{}
		}
	}

	for _, o := range unstructuredObjects {
		if o.GetNamespace() != namespace {
			continue
		}
		if _, ok := FilteredResources[o.GetObjectKind().GroupVersionKind().GroupKind()]; ok {
			continue
		}
		objects = append(objects, o)
	}
	return objects, nil
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
	return supportedProviders
}

// MergeGatewayResources accept multiple GatewayResources and create a unique Resource struct
// built as follows:
//   - GatewayClasses, *Routes, and ReferenceGrants are grouped into the same maps
//   - Gateways may have the same NamespaceName even if they come from different
//     ingresses, as they have a their GatewayClass' name as name. For this reason,
//     if there are mutiple gateways named the same, their listeners are merged into
//     a unique Gateway.
//
// This behavior is likely to change after https://github.com/kubernetes-sigs/gateway-api/pull/1863 takes place.
func MergeGatewayResources(gatewayResources ...GatewayResources) (GatewayResources, field.ErrorList) {
	mergedGatewayResources := GatewayResources{
		Gateways:        make(map[types.NamespacedName]gatewayv1.Gateway),
		GatewayClasses:  make(map[types.NamespacedName]gatewayv1.GatewayClass),
		HTTPRoutes:      make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		TLSRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		UDPRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.UDPRoute),
		ReferenceGrants: make(map[types.NamespacedName]gatewayv1beta1.ReferenceGrant),
	}
	var errs field.ErrorList
	mergedGatewayResources.Gateways, errs = mergeGateways(gatewayResources)
	if len(errs) > 0 {
		return GatewayResources{}, errs
	}
	for _, gr := range gatewayResources {
		maps.Copy(mergedGatewayResources.GatewayClasses, gr.GatewayClasses)
		maps.Copy(mergedGatewayResources.HTTPRoutes, gr.HTTPRoutes)
		maps.Copy(mergedGatewayResources.TLSRoutes, gr.TLSRoutes)
		maps.Copy(mergedGatewayResources.TCPRoutes, gr.TCPRoutes)
		maps.Copy(mergedGatewayResources.UDPRoutes, gr.UDPRoutes)
		maps.Copy(mergedGatewayResources.ReferenceGrants, gr.ReferenceGrants)
	}
	return mergedGatewayResources, errs
}

func mergeGateways(gatewaResources []GatewayResources) (map[types.NamespacedName]gatewayv1.Gateway, field.ErrorList) {
	newGateways := map[types.NamespacedName]gatewayv1.Gateway{}
	errs := field.ErrorList{}

	for _, gr := range gatewaResources {
		for _, g := range gr.Gateways {
			nn := types.NamespacedName{Namespace: g.Namespace, Name: g.Name}
			if existingGateway, ok := newGateways[nn]; ok {
				g.Spec.Listeners = append(g.Spec.Listeners, existingGateway.Spec.Listeners...)
				g.Spec.Addresses = append(g.Spec.Addresses, existingGateway.Spec.Addresses...)
			}
			newGateways[nn] = g
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

	return newGateways, errs
}
