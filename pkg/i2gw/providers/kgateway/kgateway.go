package kgateway

import (
	"context"
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"

	kgwv1a1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const Name = "kgateway"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

type Provider struct {
	storage          *storage
	reader           resourceReader
	irConverter      resourcesToIRConverter
	gatewayConverter irToGatewayResourcesConverter
}

func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	// Add BackendConfig and FrontendConfig to Schema when reading in-cluster
	// so these resources can be recognized.
	if conf.Client != nil {
		if err := kgwv1a1.Install(conf.Client.Scheme()); err != nil {
			notify(notifications.ErrorNotification, "Failed to add Kgateway v1alpha1 Scheme")
		}
	}

	return &Provider{
		storage:          newResourcesStorage(),
		reader:           newResourceReader(conf),
		irConverter:      newResourcesToIRConverter(conf),
		gatewayConverter: newIRToGatewayResourcesConverter(),
	}
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
