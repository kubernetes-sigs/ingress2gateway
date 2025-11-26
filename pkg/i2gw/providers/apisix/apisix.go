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

package apisix

import (
	"context"
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// The Name of the provider.
const Name = "apisix"
const ApisixIngressClass = "apisix"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	storage                *storage
	resourceReader         *resourceReader
	resourcesToIRConverter *resourcesToIRConverter
}

// NewProvider constructs and returns the apisix implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:                newResourcesStorage(),
		resourceReader:         newResourceReader(conf),
		resourcesToIRConverter: newResourcesToIRConverter(),
	}
}

// ToIR converts stored Apisix API entities to intermediate.IR
// including the apisix specific features.
func (p *Provider) ToIR() (provider_intermediate.IR, field.ErrorList) {
	return p.resourcesToIRConverter.convertToIR(p.storage)
}

func (p *Provider) ToGatewayResources(ir provider_intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	return common.ToGatewayResources(ir)
}

func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.resourceReader.readResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read resources from cluster: %w", err)
	}

	p.storage = storage
	return nil
}

func (p *Provider) ReadResourcesFromFile(_ context.Context, filename string) error {
	storage, err := p.resourceReader.readResourcesFromFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read resources from file: %w", err)
	}

	p.storage = storage
	return nil
}
