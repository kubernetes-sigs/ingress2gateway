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
	"context"
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// The ProviderName returned to the provider's registry.
const ProviderName = "istio"

func init() {
	i2gw.ProviderConstructorByName[ProviderName] = NewProvider
}

type Provider struct {
	storage                *storage
	reader                 reader
	resourcesToIRConverter resourcesToIRConverter
}

// NewProvider returns the istio implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:                newResourcesStorage(),
		reader:                 newResourceReader(conf),
		resourcesToIRConverter: newResourcesToIRConverter(),
	}
}

// ToIR converts stored Istio API entities to intermediate.IR
// K8S Ingress resources are not needed, only Istio-based are converted
func (p *Provider) ToIR() (intermediate.IR, field.ErrorList) {
	return p.resourcesToIRConverter.convertToIR(p.storage)
}

func (p *Provider) ToGatewayResources(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	return common.ToGatewayResources(ir)
}

func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.reader.readResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read resources from cluster: %w", err)
	}

	p.storage = storage
	return nil
}

func (p *Provider) ReadResourcesFromFile(ctx context.Context, filename string) error {
	storage, err := p.reader.readResourcesFromFile(ctx, filename)
	if err != nil {
		return fmt.Errorf("failed to read resources from file: %w", err)
	}
	p.storage = storage
	return nil
}
