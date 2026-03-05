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

package gce

import (
	"context"
	"fmt"
	"io"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
	frontendconfigv1beta1 "k8s.io/ingress-gce/pkg/apis/frontendconfig/v1beta1"
)

const ProviderName = "gce"

const (
	GatewayClassNameFlag = "gateway-class-name"
)

func init() {
	i2gw.ProviderConstructorByName[ProviderName] = NewProvider
	i2gw.RegisterProviderSpecificFlag("gce", i2gw.ProviderSpecificFlag{
		Name:         GatewayClassNameFlag,
		Description:  "The name of the GatewayClass to use for the Gateway",
		DefaultValue: "",
	})
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	storage     *storage
	reader      reader
	irConverter resourcesToIRConverter
}

func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	// Add BackendConfig and FrontendConfig to Schema when reading in-cluster
	// so these resources can be recognized.
	if conf.Client != nil {
		if err := backendconfigv1.AddToScheme(conf.Client.Scheme()); err != nil {
			notify(notifications.ErrorNotification, "Failed to add v1 BackendConfig Scheme")
		}
		if err := frontendconfigv1beta1.AddToScheme(conf.Client.Scheme()); err != nil {
			notify(notifications.ErrorNotification, "Failed to add v1beta1 FrontendConfig Scheme")
		}
	}
	return &Provider{
		storage:     newResourcesStorage(),
		reader:      newResourceReader(conf),
		irConverter: newResourcesToIRConverter(conf),
	}
}

func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.reader.readResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read gce resources from cluster: %w", err)
	}

	p.storage = storage
	return nil
}

func (p *Provider) ReadResourcesFromFile(_ context.Context, reader io.Reader) error {
	storage, err := p.reader.readResourcesFromFile(reader)
	if err != nil {
		return fmt.Errorf("failed to read gce resources from file: %w", err)
	}
	p.storage = storage
	return nil
}

// ToIR converts stored Ingress GCE API entities to providerir.IR including the
// ingress-gce specific features.
func (p *Provider) ToIR() (emitterir.EmitterIR, field.ErrorList) {
	ir, errs := p.irConverter.convertToIR(p.storage)
	return providerir.ToEmitterIR(ir), errs
}
