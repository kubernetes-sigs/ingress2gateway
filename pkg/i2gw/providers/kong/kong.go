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

package kong

import (
	"context"
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// The Name of the provider.
const Name = "kong"
const KongIngressClass = "kong"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	storage        *storage
	resourceReader *resourceReader
	converter      *converter
}

// NewProvider constructs and returns the kong implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:        newResourcesStorage(),
		resourceReader: newResourceReader(conf),
		converter:      newConverter(),
	}
}

// ToGatewayAPI converts the received i2gw.InputResources to i2gw.GatewayResources
// including the kong specific features.
func (p *Provider) ToGatewayAPI(_ i2gw.InputResources) (i2gw.GatewayResources, field.ErrorList) {
	return p.converter.convert(p.storage)
}

func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.resourceReader.readResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read resources from cluster: %w", err)
	}

	p.storage = storage
	return nil
}

func (p *Provider) ReadResourcesFromFile(ctx context.Context, filename string) error {
	storage, err := p.resourceReader.readResourcesFromFile(ctx, filename)
	if err != nil {
		return fmt.Errorf("failed to read resources from file: %w", err)
	}

	p.storage = storage
	return nil
}
