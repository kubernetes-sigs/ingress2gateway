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

package cilium

import (
	"context"
	"fmt"
	"io"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// The Name of the provider.
const Name = "cilium"
const CiliumIngressClass = "cilium"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	storage                *storage
	resourceReader         *resourceReader
	resourcesToIRConverter *resourcesToIRConverter
	notify                 notifications.NotifyFunc
}

// NewProvider constructs and returns the cilium implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	notify := conf.Report.Notifier(Name)

	return &Provider{
		storage:                newResourcesStorage(),
		resourceReader:         newResourceReader(conf),
		resourcesToIRConverter: newResourcesToIRConverter(notify),
		notify:                 notify,
	}
}

// ToIR converts stored Cilium API entities to emitterir.IR
// including the cilium specific features.
func (p *Provider) ToIR() (emitterir.EmitterIR, field.ErrorList) {
	ir, errs := p.resourcesToIRConverter.convertToIR(p.storage)
	return providerir.ToEmitterIR(ir), errs
}

func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.resourceReader.readResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read resources from cluster: %w", err)
	}

	p.storage = storage
	return nil
}

func (p *Provider) ReadResourcesFromFile(_ context.Context, reader io.Reader) error {
	storage, err := p.resourceReader.readResourcesFromFile(reader)
	if err != nil {
		return fmt.Errorf("failed to read resources from file: %w", err)
	}

	p.storage = storage
	return nil
}
