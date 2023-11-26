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

package istio

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// The ProviderName returned to the provider's registry.
const ProviderName = "istio"

func init() {
	i2gw.ProviderConstructorByName[ProviderName] = NewProvider
}

type Provider struct {
	storage   storage
	reader    reader
	converter converter
}

// NewProvider returns the istio implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:   newResourcesStorage(),
		reader:    newResourceReader(conf.Client),
		converter: newConverter(),
	}
}

// ToGatewayAPI converts stored Istio API entities to i2gw.GatewayResources
// K8S Ingress resources are not needed, only Istio-based are converted
// TODO(#100) - add istio conversion logic
func (p *Provider) ToGatewayAPI(_ i2gw.InputResources) (i2gw.GatewayResources, field.ErrorList) {
	p.converter.convert(p.storage)
	return i2gw.GatewayResources{}, field.ErrorList{field.Forbidden(field.NewPath(""), "conversion is WIP")}
}

func (p *Provider) FetchResourcesFromCluster(ctx context.Context) error {
	storage, err := p.reader.fetchResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read resources from cluster: %w", err)
	}

	p.storage = *storage
	return nil
}

func (p *Provider) FetchResourcesFromFile(_ context.Context, filename string) error {
	stream, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %v: %w", filename, err)
	}

	reader := bytes.NewReader(stream)
	unstructuredObjects, err := i2gw.ExtractObjectsFromReader(reader)
	if err != nil {
		return fmt.Errorf("failed to extract objects: %w", err)
	}

	storage, err := p.reader.readUnstructuredObjects(unstructuredObjects)
	if err != nil {
		return fmt.Errorf("failed to read unstructured objects: %w", err)
	}

	p.storage = *storage
	return nil
}
