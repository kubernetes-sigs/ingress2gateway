package kgateway

import (
	"context"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/sets"
)

const NginxIngressClass = "nginx"

var supportedIngressClass = sets.New(NginxIngressClass, "")

// resourceReader implements the i2gw.CustomResourceReader interface.
type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) resourceReader {
	return resourceReader{
		conf: conf,
	}
}

func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	// read cilium related resources from cluster.
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, supportedIngressClass)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	services, err := common.ReadServicesFromCluster(ctx, r.conf.Client)
	if err != nil {
		return nil, err
	}
	storage.Services = services
	storage.ServicePorts = common.GroupServicePortsByPortName(services)
	return storage, nil
}

func (r *resourceReader) readResourcesFromFile(filename string) (*storage, error) {
	// read cilium related resources from file.
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromFile(filename, r.conf.Namespace, supportedIngressClass)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	services, err := common.ReadServicesFromFile(filename, r.conf.Namespace)
	if err != nil {
		return nil, err
	}
	storage.Services = services
	storage.ServicePorts = common.GroupServicePortsByPortName(services)
	return storage, nil
}
