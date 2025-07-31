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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/sets"
)

// converter implements the i2gw.CustomResourceReader interface.
type resourceReader struct {
	conf         *i2gw.ProviderConf
	ingressClass string
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	var ingressClass string

	if ps := conf.ProviderSpecificFlags[Name]; ps != nil {
		ingressClass = ps[NginxIngressClassFlag]
	}

	return &resourceReader{
		conf:         conf,
		ingressClass: ingressClass,
	}
}

func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, sets.New(r.ingressClass))
	if err != nil {
		return nil, err
	}
	storage.Ingresses.FromMap(ingresses)

	services, err := common.ReadServicesFromCluster(ctx, r.conf.Client)
	if err != nil {
		return nil, err
	}
	storage.ServicePorts = common.GroupServicePortsByPortName(services)
	return storage, nil
}

func (r *resourceReader) readResourcesFromFile(filename string) (*storage, error) {
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromFile(filename, r.conf.Namespace, sets.New(r.ingressClass))
	if err != nil {
		return nil, err
	}
	storage.Ingresses.FromMap(ingresses)

	services, err := common.ReadServicesFromFile(filename, r.conf.Namespace)
	if err != nil {
		return nil, err
	}
	storage.ServicePorts = common.GroupServicePortsByPortName(services)
	return storage, nil
}
