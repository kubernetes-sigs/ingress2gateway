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
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ConstructIngressesFromCluster(cl client.Client, ingressList *networkingv1.IngressList) error {
	err := cl.List(context.Background(), ingressList)
	if err != nil {
		return fmt.Errorf("failed to get ingresses from the cluster: %w", err)
	}
	return nil
}

func Ingresses2GatewaysAndHTTPRoutes(ingresses []networkingv1.Ingress) ([]gatewayv1beta1.HTTPRoute, []gatewayv1beta1.Gateway, field.ErrorList) {
	var gateways []gatewayv1beta1.Gateway
	var httpRoutes []gatewayv1beta1.HTTPRoute
	var errs field.ErrorList

	providerByName := constructProviders(&ProviderConf{})
	if len(providerByName) == 0 {
		errs = append(errs, field.Invalid(nil, "", "no providers"))
		return nil, nil, errs
	}

	for name, provider := range providerByName {
		var customResources interface{}

		if err := provider.ReadResourcesFromCluster(context.Background(), &customResources); err != nil {
			errs = append(errs, field.Invalid(nil, "", fmt.Sprintf("failed to read %s resources from the cluster: %w", name, err)))
			return nil, nil, errs
		}

		// TODO: Open a new issue - the ingress-nginx provider contains conversion logic which
		// will be common to all the other providers. Extract it from there and create a common
		// ingress conversion function.
		convertedHTTPRoutes, convertedGateways, conversionErrs := provider.ConvertHTTPRoutes(ingresses, customResources)
		httpRoutes = append(httpRoutes, convertedHTTPRoutes...)
		gateways = append(gateways, convertedGateways...)
		errs = append(errs, conversionErrs...)
	}

	return httpRoutes, gateways, nil
}

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

// constructProviders constructs a map of concrete Provider implementations
// by their ProviderName.
//
// TODO: Open a new issue - let users filter by provider name.
func constructProviders(conf *ProviderConf) map[ProviderName]Provider {
	providerByName := make(map[ProviderName]Provider, len(ProviderConstructorByName))

	for name, newProviderFunc := range ProviderConstructorByName {
		providerByName[name] = newProviderFunc(conf)
	}

	return providerByName
}

func outputResult(printer printers.ResourcePrinter, httpRoutes []gatewayv1beta1.HTTPRoute, gateways []gatewayv1beta1.Gateway) {
	for i := range gateways {
		err := printer.PrintObj(&gateways[i], os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", gateways[i].Name, err)
		}
	}

	for i := range httpRoutes {
		err := printer.PrintObj(&httpRoutes[i], os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", httpRoutes[i].Name, err)
		}
	}
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
