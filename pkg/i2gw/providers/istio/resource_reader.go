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

package istio

import (
	"context"
	"fmt"
	"log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reader struct {
	k8sClient client.Client
}

func newResourceReader(k8sClient client.Client) reader {
	return reader{
		k8sClient: k8sClient,
	}
}

func (r *reader) fetchResourcesFromCluster(ctx context.Context) (*storage, error) {
	res := newResourcesStorage()

	gateways, err := r.readGatewaysFromCluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read gateways: %w", err)
	}

	res.Gateways = gateways

	virtualServices, err := r.readVirtualServicesFromCluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read virtual services: %w", err)
	}

	res.VirtualServices = virtualServices

	return &res, nil
}

func (r *reader) readUnstructuredObjects(objects []*unstructured.Unstructured) (*storage, error) {
	res := newResourcesStorage()

	for _, obj := range objects {
		if obj.GetAPIVersion() != APIVersion {
			log.Printf("%v provider: skipped resource with unsupported APIVersion: %v", ProviderName, obj.GetAPIVersion())
			continue
		}

		switch objKind := obj.GetKind(); objKind {
		case GatewayKind:
			var gw gateway
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &gw); err != nil {
				return nil, fmt.Errorf("failed to parse istio gateway object: %w", err)
			}
			res.Gateways[types.NamespacedName{
				Namespace: gw.Namespace,
				Name:      gw.Name,
			}] = &gw

		case VirtualServiceKind:
			var vs virtualService
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &vs); err != nil {
				return nil, fmt.Errorf("failed to parse istio virtual service object: %w", err)
			}

			res.VirtualServices[types.NamespacedName{
				Namespace: vs.Namespace,
				Name:      vs.Name,
			}] = &vs
		default:
			log.Printf("%v provider: skipped resource with unsupported Kind: %v", ProviderName, objKind)
			continue
		}
	}

	return &res, nil
}

func (r *reader) readGatewaysFromCluster(ctx context.Context) (map[types.NamespacedName]*gateway, error) {
	gatewayList := &unstructured.UnstructuredList{}
	gatewayList.SetAPIVersion(APIVersion)
	gatewayList.SetKind(GatewayKind)

	err := r.k8sClient.List(ctx, gatewayList)
	if err != nil {
		return nil, fmt.Errorf("failed to list istio gateways: %w", err)
	}

	res := map[types.NamespacedName]*gateway{}
	for _, obj := range gatewayList.Items {
		var gw gateway
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &gw); err != nil {
			return nil, fmt.Errorf("failed to parse istio gateway object: %w", err)
		}

		res[types.NamespacedName{
			Namespace: gw.Namespace,
			Name:      gw.Name,
		}] = &gw
	}

	return res, nil
}

func (r *reader) readVirtualServicesFromCluster(ctx context.Context) (map[types.NamespacedName]*virtualService, error) {
	virtualServicesList := &unstructured.UnstructuredList{}
	virtualServicesList.SetAPIVersion(APIVersion)
	virtualServicesList.SetKind(VirtualServiceKind)

	err := r.k8sClient.List(ctx, virtualServicesList)
	if err != nil {
		return nil, fmt.Errorf("failed to list istio virtual services: %w", err)
	}

	res := map[types.NamespacedName]*virtualService{}

	for _, obj := range virtualServicesList.Items {
		var vs virtualService
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &vs); err != nil {
			return nil, fmt.Errorf("failed to parse istio virtual service object: %w", err)
		}

		res[types.NamespacedName{
			Namespace: vs.Namespace,
			Name:      vs.Name,
		}] = &vs
	}

	return res, nil
}
