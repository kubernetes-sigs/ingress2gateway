/*
Copyright 2025 The Kubernetes Authors.

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

package nginx

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
)

const Name = "nginx"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

type Provider struct {
	*storage
	*resourceReader
	*resourcesToIRConverter
}

// NewProvider constructs and returns the nginx implementation of i2gw.Provider
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		resourceReader:            newResourceReader(conf),
		resourcesToIRConverter:    newResourcesToIRConverter(),
	}
}

// ReadResourcesFromCluster reads resources from the Kubernetes cluster
func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.readResourcesFromCluster(ctx)
	if err != nil {
		return err
	}
	p.storage = storage
	return nil
}

// ReadResourcesFromFile reads resources from a YAML file
func (p *Provider) ReadResourcesFromFile(_ context.Context, filename string) error {
	storage, err := p.readResourcesFromFile(filename)
	if err != nil {
		return err
	}
	p.storage = storage
	return nil
}

// ToIR converts the provider resources to intermediate representation
func (p *Provider) ToIR() (emitter_intermediate.IR, field.ErrorList) {
	ir, errs := p.resourcesToIRConverter.convert(p.storage)
	return provider_intermediate.ToEmitterIR(ir), errs
}
