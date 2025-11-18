package kgateway

import (
	"context"
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const Name = "kgateway"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

type Provider struct {
	storage          *storage
	reader           *resourceReader
	irConverter      *resourcesToIRConverter
	gatewayConverter *irToGatewayResourcesConverter
}

// ReadResourcesFromCluster implements i2gw.Provider.

func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.reader.readResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read kgateway resources from cluster: %w", err)
	}

	p.storage = storage
	return nil
}

// ReadResourcesFromFile implements i2gw.Provider.

func (p *Provider) ReadResourcesFromFile(_ context.Context, filename string) error {
	storage, err := p.reader.readResourcesFromFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read kgateway resources from file: %w", err)
	}
	p.storage = storage
	return nil
}

// ToGatewayResources implements i2gw.Provider.
func (p *Provider) ToGatewayResources(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	return p.gatewayConverter.irToGateway(ir)
}

// ToIR implements i2gw.Provider.
func (p *Provider) ToIR() (intermediate.IR, field.ErrorList) {
	return p.irConverter.convertToIR(p.storage)
}

func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:     newResourcesStorage(),
		reader:      newResourceReader(conf),
		irConverter: newResourcesToIRConverter(conf),
	}
}
