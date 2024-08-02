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
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
	backendconfigv1beta1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1beta1"
)

// GCE supports the following Ingress Class values:
// 1. "gce", for external Ingress
// 2. "gce-internal", for internal Ingress
// 3. "", which defaults to external Ingress
var supportedGCEIngressClass = sets.New(gceIngressClass, gceL7ILBIngressClass, "")

const (
	IngressKind = "Ingress"
)

// reader implements the i2gw.CustomResourceReader interface.
type reader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) reader {
	return reader{
		conf: conf,
	}
}

func (r *reader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, supportedGCEIngressClass)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	services, err := r.readServicesFromCluster(ctx)
	if err != nil {
		return nil, err
	}
	storage.Services = services

	backendConfigs, err := r.readBackendConfigsFromCluster(ctx)
	if err != nil {
		return nil, err
	}
	storage.BackendConfigs = backendConfigs
	return storage, nil
}

func (r *reader) readResourcesFromFile(filename string) (*storage, error) {
	stream, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", filename, err)
	}

	unstructuredObjects, err := common.ExtractObjectsFromReader(bytes.NewReader(stream), r.conf.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	storage, err := r.readUnstructuredObjects(unstructuredObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to read unstructured objects: %w", err)
	}

	return storage, nil
}

func (r *reader) readServicesFromCluster(ctx context.Context) (map[types.NamespacedName]*apiv1.Service, error) {
	var serviceList apiv1.ServiceList
	err := r.conf.Client.List(ctx, &serviceList)
	if err != nil {
		return nil, fmt.Errorf("failed to get services from the cluster: %w", err)
	}
	services := make(map[types.NamespacedName]*apiv1.Service)
	for i, service := range serviceList.Items {
		services[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}] = &serviceList.Items[i]
	}
	return services, nil
}

func (r *reader) readBackendConfigsFromCluster(ctx context.Context) (map[types.NamespacedName]*backendconfigv1.BackendConfig, error) {
	var backendConfigList backendconfigv1.BackendConfigList
	err := r.conf.Client.List(ctx, &backendConfigList)
	if err != nil {
		return nil, fmt.Errorf("failed to get backendConfigs from the cluster: %w", err)
	}
	backendConfigs := make(map[types.NamespacedName]*backendconfigv1.BackendConfig)
	for i, backendConfig := range backendConfigList.Items {
		backendConfigs[types.NamespacedName{Namespace: backendConfig.Namespace, Name: backendConfig.Name}] = &backendConfigList.Items[i]
	}
	return backendConfigs, nil
}

func (r *reader) readUnstructuredObjects(objects []*unstructured.Unstructured) (*storage, error) {
	res := newResourcesStorage()

	ingresses := make(map[types.NamespacedName]*networkingv1.Ingress)
	services := make(map[types.NamespacedName]*apiv1.Service)
	backendConfigs := make(map[types.NamespacedName]*backendconfigv1.BackendConfig)
	betaBackendConfigs := make(map[types.NamespacedName]*backendconfigv1beta1.BackendConfig)

	for _, f := range objects {
		if f.GroupVersionKind().Empty() {
			continue
		}

		if f.GetKind() == "Ingress" {
			var ingress networkingv1.Ingress
			err := runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &ingress)
			if err != nil {
				return nil, err
			}
			if !supportedGCEIngressClass.Has(common.GetIngressClass(ingress)) {
				continue
			}
			ingresses[types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}] = &ingress
		}
		if f.GetAPIVersion() == "v1" && f.GetKind() == "Service" {
			var service apiv1.Service
			err := runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &service)
			if err != nil {
				return nil, err
			}
			services[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}] = &service
		}
		if f.GetAPIVersion() == "cloud.google.com/v1" && f.GetKind() == "BackendConfig" {
			var backendConfig backendconfigv1.BackendConfig
			err := runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &backendConfig)
			if err != nil {
				return nil, err
			}
			backendConfigs[types.NamespacedName{Namespace: backendConfig.Namespace, Name: backendConfig.Name}] = &backendConfig
		}
		if f.GetAPIVersion() == "cloud.google.com/v1beta1" && f.GetKind() == "BackendConfig" {
			var betaBackendConfig backendconfigv1beta1.BackendConfig
			err := runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &betaBackendConfig)
			if err != nil {
				return nil, err
			}
			betaBackendConfigs[types.NamespacedName{Namespace: betaBackendConfig.Namespace, Name: betaBackendConfig.Name}] = &betaBackendConfig
		}
	}
	res.Ingresses = ingresses
	res.Services = services
	res.BackendConfigs = backendConfigs
	res.BetaBackendConfigs = betaBackendConfigs
	return res, nil
}
