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

package kong

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kongv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
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

// -----------------------------------------------------------------------------
// readers - all objects
// -----------------------------------------------------------------------------

func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourceStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, KongIngressClass)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	tcpIngresses, err := r.readTCPIngressesFromCluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read TCPIngresses: %w", err)
	}
	storage.TCPIngresses = tcpIngresses

	return storage, nil
}

func (r *resourceReader) readResourcesFromFile(filename string) (*storage, error) {
	storage := newResourceStorage()

	ingresses, err := common.ReadIngressesFromFile(filename, r.conf.Namespace, KongIngressClass)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	tcpIngresses, err := r.readTCPIngressesFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read TCPIngresses: %w", err)
	}
	storage.TCPIngresses = tcpIngresses

	return storage, nil
}

// -----------------------------------------------------------------------------
// readers - TCPIngress
// -----------------------------------------------------------------------------

func (r *resourceReader) readTCPIngressesFromCluster(ctx context.Context) ([]kongv1beta1.TCPIngress, error) {
	tcpIngressList := &unstructured.UnstructuredList{}
	tcpIngressList.SetGroupVersionKind(tcpIngressGVK)

	err := r.conf.Client.List(ctx, tcpIngressList)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			klog.Warningf("couldn't find %s CRD, it is likely not installed in the cluster", tcpIngressGVK.GroupKind().String())
			return []kongv1beta1.TCPIngress{}, nil
		}
		return nil, fmt.Errorf("failed to list %s: %w", tcpIngressGVK.GroupKind().String(), err)
	}

	tcpIngresses := []kongv1beta1.TCPIngress{}
	for _, obj := range tcpIngressList.Items {
		var tcpIngress kongv1beta1.TCPIngress
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &tcpIngress); err != nil {
			return nil, fmt.Errorf("failed to parse Kong TCPIngress object: %w", err)
		}

		tcpIngresses = append(tcpIngresses, tcpIngress)
	}

	return tcpIngresses, nil
}

func (r *resourceReader) readTCPIngressesFromFile(filename string) ([]kongv1beta1.TCPIngress, error) {
	stream, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(stream)
	objs, err := common.ExtractObjectsFromReader(reader, r.conf.Namespace)
	if err != nil {
		return nil, err
	}

	tcpIngresses := []kongv1beta1.TCPIngress{}
	for _, f := range objs {
		if r.conf.Namespace != "" && f.GetNamespace() != r.conf.Namespace {
			continue
		}
		if !f.GroupVersionKind().Empty() &&
			f.GroupVersionKind() == tcpIngressGVK {
			tcpIngress := &kongv1beta1.TCPIngress{}
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), tcpIngress)
			if err != nil {
				return nil, err
			}
			tcpIngresses = append(tcpIngresses, *tcpIngress)
		}
	}

	return tcpIngresses, nil
}
