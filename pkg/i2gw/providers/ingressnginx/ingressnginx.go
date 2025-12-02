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

package ingressnginx

import (
	"context"
	"fmt"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// The Name of the provider.
const Name = "ingress-nginx"
const NginxIngressClass = "nginx"
const NginxIngressClassFlag = "ingress-class"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
	i2gw.RegisterProviderSpecificFlag(Name, i2gw.ProviderSpecificFlag{
		Name:         "ingress-class",
		Description:  "The name of the ingress class to select. Defaults to 'nginx'",
		DefaultValue: NginxIngressClass,
	})
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	storage                *storage
	resourceReader         *resourceReader
	resourcesToIRConverter *resourcesToIRConverter
}

// NewProvider constructs and returns the ingress-nginx implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:                newResourcesStorage(),
		resourceReader:         newResourceReader(conf),
		resourcesToIRConverter: newResourcesToIRConverter(),
	}
}

// ToIR converts stored Ingress-Nginx API entities to intermediate.IR
// including the ingress-nginx specific features.
func (p *Provider) ToIR() (intermediate.IR, field.ErrorList) {
	return p.resourcesToIRConverter.convert(p.storage)
}

func (p *Provider) ToGatewayResources(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
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
