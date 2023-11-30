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
)

// converter implements the i2gw.CustomResourceFetcher interface.
type resourceFetcher struct {
	conf *i2gw.ProviderConf
}

// newResourceFetcher returns a resourceFetcher instance.
func newResourceFetcher(conf *i2gw.ProviderConf) *resourceFetcher {
	return &resourceFetcher{
		conf: conf,
	}
}

func (r *resourceFetcher) FetchResourcesFromCluster(_ context.Context) error {
	// ingress-nginx does not have any CRDs.
	return nil
}

func (r *resourceFetcher) FetchResourcesFromFile(_ context.Context, _ string) error {
	// ingress-nginx does not have any CRDs.
	return nil
}
