/*
Copyright 2024 The Kubernetes Authors.

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

package openapi

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

// The ProviderName returned to the provider's registry.
const ProviderName = "openapi"

type Provider struct {
	storage   *storage
	reader    *resourceReader
	converter *converter
}

var _ i2gw.Provider = &Provider{}

// NewProvider returns an implementation of i2gw.Provider that converts OpenAPI specs to Gateway API resources.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:   newResourceStorage(),
		reader:    newResourceReader(conf),
		converter: newConverter(conf),
	}
}

// ReadResourcesFromCluster reads OpenAPI specs stored in the Kubernetes cluster. UNIMPLEMENTED.
func (p *Provider) ReadResourcesFromCluster(_ context.Context) error {
	return fmt.Errorf("provider does not support reading resources from cluster")
}

// ReadResourcesFromFile reads OpenAPI specs from a JSON or YAML file.
func (p *Provider) ReadResourcesFromFile(ctx context.Context, filename string) error {
	storage, err := p.reader.readResourcesFromFile(ctx, filename)
	if err != nil {
		return fmt.Errorf("failed to read resources from file: %w", err)
	}
	p.storage = storage
	return nil
}

// ToGatewayAPI converts stored OpenAPI specs to Gateway API resources.
func (p *Provider) ToGatewayAPI(_ i2gw.InputResources) (i2gw.GatewayResources, field.ErrorList) {
	return p.converter.convert(p.storage)
}

func init() {
	i2gw.ProviderConstructorByName[ProviderName] = NewProvider
}
