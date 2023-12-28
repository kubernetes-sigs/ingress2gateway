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
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// converter implements the i2gw.CustomResourceReader interface.
type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	return &resourceReader{
		conf: conf,
	}
}

func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourcesStorage()

	var ingressList networkingv1.IngressList
	err := r.conf.Client.List(ctx, &ingressList)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingresses from the cluster: %w", err)
	}

	for i, ingress := range ingressList.Items {
		if common.GetIngressClass(ingress) != NginxIngressClass {
			continue
		}
		storage.Ingresses[types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}] = &ingressList.Items[i]
	}

	return storage, nil
}

func (r *resourceReader) readResourcesFromFile(_ context.Context, filename string) (*storage, error) {
	storage := newResourcesStorage()
	stream, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", filename, err)
	}

	unstructuredObjects, err := i2gw.ExtractObjectsFromReader(bytes.NewReader(stream), r.conf.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	for _, f := range unstructuredObjects {
		if !f.GroupVersionKind().Empty() && f.GroupVersionKind().Kind == "Ingress" {
			var i networkingv1.Ingress
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &i)
			if err != nil {
				return nil, err
			}
			if common.GetIngressClass(i) != NginxIngressClass {
				continue
			}
			storage.Ingresses[types.NamespacedName{Namespace: i.Namespace, Name: i.Name}] = &i
		}

	}
	return storage, nil
}
