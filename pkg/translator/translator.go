/*
Copyright © 2023 Kubernetes Authors

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
package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

type Translator struct {
	Options
}

func New(options Options) *Translator {
	return &Translator{
		options,
	}
}

type Options struct {
	Writer       io.Writer
	Mode         string
	FilePath     string
	OutputType   string
	ResourceType string
	Provider     IngressProvider
}

func NewOptions(w io.Writer, mode, filePath, outputType, resourceType, provider string) Options {
	return Options{
		Writer:       w,
		FilePath:     filePath,
		OutputType:   outputType,
		ResourceType: resourceType,
		Mode:         mode,
		Provider:     IngressProvider(provider),
	}
}

func (t Translator) Run() error {
	if err := t.validate(); err != nil {
		return err
	}

	var (
		result ResultResources
		errs   []error
	)

	ingresses, err := t.ConstructIngresses(t.Mode, t.FilePath)
	if err != nil {
		return err
	}

	result, errs = t.Convert(t.Provider, ingresses)
	if len(errs) != 0 {
		fmt.Fprintln(t.Writer, errs)
	}

	data, err := t.Marshal(result, t.OutputType, t.ResourceType)
	if err != nil {
		return err
	}

	fmt.Fprintln(t.Writer, string(data))

	return nil
}

func (t Translator) validate() error {
	if !isValidResourceType(GWAPIResourceType(t.ResourceType)) {
		return fmt.Errorf("%s is not a valid output type. %s", t.ResourceType, GetValidResourceTypesStr())
	}

	return nil
}

func (t Translator) ConstructIngresses(mode, file string) ([]networkingv1.Ingress, error) {
	if mode == LocalMode {
		inBytes, err := getInputBytes(file)
		if err != nil {
			return nil, fmt.Errorf("unable to read input file: %w", err)
		}

		ingresses, err := kubernetesYAMLToResources(string(inBytes))
		if len(ingresses) == 0 {
			return nil, fmt.Errorf("no ingress resources provided")
		}

		if err != nil {
			return nil, err
		}

		return ingresses, nil
	}

	cl, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	ingressList := &networkingv1.IngressList{}

	err = cl.List(context.Background(), ingressList)
	if err != nil {
		fmt.Printf("failed to list ingresses: %v\n", err)
		os.Exit(1)
	}

	return ingressList.Items, nil
}

func (t Translator) Convert(provider IngressProvider, ingresses []networkingv1.Ingress) (ResultResources, []error) {
	aggregator := NewAggregator()

	for _, ingress := range ingresses {
		aggregator.addIngress(provider, ingress)
	}

	return aggregator.convert()
}

func (t Translator) Marshal(result ResultResources, output, resourceType string) ([]byte, error) {
	var resData []byte
	var err error

	if output == yamlOutput {
		switch resourceType {
		case string(AllGWAPIResourceType):
			resData, err = yaml.Marshal(result)
		case string(HTTPRouteGWAPIResourceType):
			resData, err = yaml.Marshal(result.HTTPRoutes)
		case string(GatewayGWAPIResourceType):
			resData, err = yaml.Marshal(result.Gateways)
		}
		if err != nil {
			return nil, err
		}
	} else if output == jsonOutput {
		switch resourceType {
		case string(AllGWAPIResourceType):
			resData, err = json.MarshalIndent(result, "", "    ")
		case string(HTTPRouteGWAPIResourceType):
			resData, err = json.MarshalIndent(result.HTTPRoutes, "", "    ")
		case string(GatewayGWAPIResourceType):
			resData, err = json.MarshalIndent(result.Gateways, "", "    ")
		}
		if err != nil {
			return nil, err
		}
	}

	return resData, nil
}
