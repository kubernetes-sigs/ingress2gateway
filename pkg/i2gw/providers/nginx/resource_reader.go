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
	"io"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx/annotations"
)

type resourceReader struct {
	conf           *i2gw.ProviderConf
	ingressClasses sets.Set[string]
}

// newResourceReader returns a resourceReader instance
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	ingressClass := annotations.NginxIngressClass
	if ps := conf.ProviderSpecificFlags[Name]; ps != nil && ps[NginxIngressClassFlag] != "" {
		ingressClass = ps[NginxIngressClassFlag]
	}

	return &resourceReader{
		conf:           conf,
		ingressClasses: sets.New(ingressClass),
	}
}

// readResourcesFromCluster reads nginx resources from the Kubernetes cluster
func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourceStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, r.ingressClasses)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	services, err := common.ReadServicesFromCluster(ctx, r.conf.Client)
	if err != nil {
		return nil, err
	}
	storage.ServicePorts = common.GroupServicePortsByPortName(services)

	return storage, nil
}

// readResourcesFromFile reads nginx resources from a YAML file
func (r *resourceReader) readResourcesFromFile(reader io.Reader) (*storage, error) {
	storage := newResourceStorage()

	ingresses, err := common.ReadIngressesFromFile(reader, r.conf.Namespace, r.ingressClasses)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	services, err := common.ReadServicesFromFile(reader, r.conf.Namespace)
	if err != nil {
		return nil, err
	}
	storage.ServicePorts = common.GroupServicePortsByPortName(services)

	return storage, nil
}
