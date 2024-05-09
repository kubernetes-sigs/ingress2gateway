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

package openapi3

import (
	"context"
	"fmt"
	"log"

	"github.com/getkin/kin-openapi/openapi3"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

const (
	// The ProviderName returned to the provider's registry.
	ProviderName = "openapi3"

	BackendFlag      = "backend"
	GatewayClassFlag = "gateway-class-name"
)

func init() {
	i2gw.ProviderConstructorByName[ProviderName] = NewProvider

	i2gw.RegisterProviderSpecificConf(ProviderName, i2gw.ProviderSpecificConf{
		Name:        BackendFlag,
		Description: "The name of the backend service to use in the HTTPRoutes",
	})

	i2gw.RegisterProviderSpecificConf(ProviderName, i2gw.ProviderSpecificConf{
		Name:        GatewayClassFlag,
		Description: "The name of the gateway class to use in the Gateways",
	})
}

type Provider struct {
	storage   Storage
	converter Converter
}

var _ i2gw.Provider = &Provider{}

// NewProvider returns an implementation of i2gw.Provider that converts OpenAPI specs to Gateway API resources.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:   NewResourceStorage(),
		converter: NewConverter(conf),
	}
}

// ReadResourcesFromCluster reads OpenAPI specs stored in the Kubernetes cluster. UNIMPLEMENTED.
func (p *Provider) ReadResourcesFromCluster(_ context.Context) error {
	return nil
}

// ReadResourcesFromFile reads OpenAPI specs from a JSON or YAML file.
func (p *Provider) ReadResourcesFromFile(ctx context.Context, filename string) error {
	spec, err := readSpecFromFile(ctx, filename)
	if err != nil {
		return fmt.Errorf("failed to read resources from file: %w", err)
	}

	p.storage.Clear()
	if spec != nil {
		p.storage.AddResource(spec)
	}

	return nil
}

// ToGatewayAPI converts stored OpenAPI specs to Gateway API resources.
func (p *Provider) ToGatewayAPI(_ i2gw.InputResources) (i2gw.GatewayResources, field.ErrorList) {
	return p.converter.Convert(p.storage)
}

func readSpecFromFile(ctx context.Context, filename string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	if err := spec.Validate(ctx); err != nil {
		log.Printf("%s provider: invalid OpenAPI 3.x spec: %v", ProviderName, err)
		return nil, nil
	}

	return spec, nil
}
