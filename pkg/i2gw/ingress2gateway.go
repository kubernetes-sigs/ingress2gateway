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
	"errors"
	"fmt"
	"io"
	"os"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ToGatewayAPIResources(ctx context.Context, namespace string, inputFile string, providers []string) ([]gatewayv1beta1.HTTPRoute, []gatewayv1beta1.Gateway, error) {
	var ingresses networkingv1.IngressList
	var gateways []gatewayv1beta1.Gateway
	var httpRoutes []gatewayv1beta1.HTTPRoute
	var providerByName map[ProviderName]Provider

	resources := InputResources{}
	if inputFile != "" {
		var err error
		providerByName, err = constructProviders(ProviderConf{}, providers)
		if err != nil {
			return nil, nil, err
		}
		if err = ConstructIngressesFromFile(&ingresses, inputFile, namespace); err != nil {
			return nil, nil, fmt.Errorf("failed to read ingresses from file: %w", err)
		}
		resources.Ingresses = ingresses.Items
		if err = readProviderResourcesFromFile(ctx, providerByName, &resources, inputFile); err != nil {
			return nil, nil, err
		}
	} else {
		conf, err := config.GetConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get client config: %w", err)
		}

		cl, err := client.New(conf, client.Options{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create client: %w", err)
		}
		cl = client.NewNamespacedClient(cl, namespace)
		providerByName, err = constructProviders(ProviderConf{
			Client: cl,
		}, providers)
		if err != nil {
			return nil, nil, err
		}
		if err = ConstructIngressesFromCluster(ctx, cl, &ingresses); err != nil {
			return nil, nil, fmt.Errorf("failed to read ingresses from cluster: %w", err)
		}
		resources.Ingresses = ingresses.Items
		if err = readProviderResourcesFromCluster(ctx, cl, providerByName, &resources); err != nil {
			return nil, nil, err
		}
	}

	var errs field.ErrorList
	for _, provider := range providerByName {
		gatewayResources, conversionErrs := provider.ToGatewayAPI(resources)
		errs = append(errs, conversionErrs...)
		for _, gateway := range gatewayResources.Gateways {
			gateways = append(gateways, gateway)
		}
		for _, route := range gatewayResources.HTTPRoutes {
			httpRoutes = append(httpRoutes, route)
		}
	}
	if len(errs) > 0 {
		return nil, nil, aggregatedErrs(errs)
	}

	return httpRoutes, gateways, nil
}

func readProviderResourcesFromFile(ctx context.Context, providerByName map[ProviderName]Provider, resources *InputResources, inputFile string) error {
	for name, provider := range providerByName {
		if err := provider.ReadResourcesFromFiles(ctx, resources.CustomResources, inputFile); err != nil {
			return fmt.Errorf("failed to read %s resources from file: %w", name, err)
		}
	}
	return nil
}

func readProviderResourcesFromCluster(ctx context.Context, cl client.Client, providerByName map[ProviderName]Provider, resources *InputResources) error {
	for name, provider := range providerByName {
		if err := provider.ReadResourcesFromCluster(ctx, cl, resources.CustomResources); err != nil {
			return fmt.Errorf("failed to read %s resources from the cluster: %w", name, err)
		}
	}
	return nil
}

func ConstructIngressesFromCluster(ctx context.Context, cl client.Client, ingressList *networkingv1.IngressList) error {
	err := cl.List(ctx, ingressList)
	if err != nil {
		return fmt.Errorf("failed to get ingresses from the cluster: %w", err)
	}
	return nil
}

// constructProviders constructs a map of concrete Provider implementations
// by their ProviderName.
func constructProviders(conf ProviderConf, providers []string) (map[ProviderName]Provider, error) {
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

// extractObjectsFromReader extracts all objects from a reader,
// which is created from YAML or JSON input files.
// It retrieves all objects, including nested ones if they are contained within a list.
func extractObjectsFromReader(reader io.Reader) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(reader, 4096)
	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := d.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		if u == nil {
			continue
		}
		objs = append(objs, u)
	}

	finalObjs := []*unstructured.Unstructured{}
	for _, obj := range objs {
		tmpObjs := []*unstructured.Unstructured{}
		if obj.IsList() {
			err := obj.EachListItem(func(object runtime.Object) error {
				unstructuredObj, ok := object.(*unstructured.Unstructured)
				if ok {
					tmpObjs = append(tmpObjs, unstructuredObj)
					return nil
				}
				return fmt.Errorf("resource list item has unexpected type")
			})
			if err != nil {
				return nil, err
			}
		} else {
			tmpObjs = append(tmpObjs, obj)
		}
		finalObjs = append(finalObjs, tmpObjs...)
	}

	return finalObjs, nil
}

// ConstructIngressesFromFile reads the inputFile in either json/yaml formats,
// then deserialize the file into Ingresses resources.
// All ingresses will be pushed into the supplied IngressList for return.
func ConstructIngressesFromFile(l *networkingv1.IngressList, inputFile string, namespace string) error {
	stream, err := os.ReadFile(inputFile)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(stream)
	objs, err := extractObjectsFromReader(reader)
	if err != nil {
		return err
	}

	for _, f := range objs {
		if namespace != "" && f.GetNamespace() != namespace {
			continue
		}
		if !f.GroupVersionKind().Empty() && f.GroupVersionKind().Kind == "Ingress" {
			var i networkingv1.Ingress
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &i)
			if err != nil {
				return err
			}
			l.Items = append(l.Items, i)
		}

	}
	return nil
}

// ConstructOtherResourcesFromFile reads the inputFile in either json/yaml formats,
// then deserialize the file into client.object resources.
func ConstructOtherResourcesFromFile(namespace string, inputFile string, providers []string) ([]*unstructured.Unstructured, error) {
	providerByName, err := constructProviders(ProviderConf{}, providers)
	if err != nil {
		return nil, err
	}

	stream, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(stream)
	objs, err := extractObjectsFromReader(reader)
	if err != nil {
		return nil, err
	}

	for _, p := range providerByName {
		objs = p.Filter(objs)
	}
	return objs, err
}

func aggregatedErrs(errs field.ErrorList) error {
	errMsg := fmt.Errorf("\n# Encountered %d errors", len(errs))
	for _, err := range errs {
		errMsg = fmt.Errorf("\n%w # %s", errMsg, err)
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
