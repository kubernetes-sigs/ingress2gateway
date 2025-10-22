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

package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	stdin     []byte
	stdinOnce sync.Once
	stdinErr  error
)

func ReadIngressesFromCluster(ctx context.Context, client client.Client, ingressClasses sets.Set[string]) (map[types.NamespacedName]*networkingv1.Ingress, error) {
	var ingressList networkingv1.IngressList
	err := client.List(ctx, &ingressList)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingresses from the cluster: %w", err)
	}

	ingresses := map[types.NamespacedName]*networkingv1.Ingress{}
	for i, ingress := range ingressList.Items {
		if !ingressClasses.Has(GetIngressClass(ingress)) {
			continue
		}
		ingresses[types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}] = &ingressList.Items[i]
	}

	return ingresses, nil
}

// readFileOrStdin reads content from a file or stdin based on the filename.
// If filename is "-", it reads from stdin, otherwise from the specified file.
// Stdin content is stored in memory after the first read to allow multiple calls.
func readFileOrStdin(filename string) ([]byte, error) {
	if filename == "-" {
		stdinOnce.Do(func() {
			stdin, stdinErr = io.ReadAll(os.Stdin)
			if stdinErr != nil {
				stdinErr = fmt.Errorf("failed to read from stdin: %w", stdinErr)
			}
		})
		return stdin, stdinErr
	}

	stream, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", filename, err)
	}
	return stream, nil
}

func ReadIngressesFromFile(filename, namespace string, ingressClasses sets.Set[string]) (map[types.NamespacedName]*networkingv1.Ingress, error) {
	stream, err := readFileOrStdin(filename)
	if err != nil {
		return nil, err
	}

	unstructuredObjects, err := ExtractObjectsFromReader(bytes.NewReader(stream), namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	ingresses := map[types.NamespacedName]*networkingv1.Ingress{}
	for _, f := range unstructuredObjects {
		if !f.GroupVersionKind().Empty() && f.GroupVersionKind().Kind == "Ingress" {
			var ingress networkingv1.Ingress
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &ingress)
			if err != nil {
				return nil, err
			}
			if !ingressClasses.Has(GetIngressClass(ingress)) {
				continue
			}
			ingresses[types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}] = &ingress
		}

	}
	return ingresses, nil
}

func ReadServicesFromCluster(ctx context.Context, client client.Client) (map[types.NamespacedName]*apiv1.Service, error) {
	var serviceList apiv1.ServiceList
	err := client.List(ctx, &serviceList)
	if err != nil {
		return nil, fmt.Errorf("failed to get services from the cluster: %w", err)
	}

	services := map[types.NamespacedName]*apiv1.Service{}
	for i, service := range serviceList.Items {
		services[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}] = &serviceList.Items[i]
	}

	return services, nil
}

func ReadServicesFromFile(filename, namespace string) (map[types.NamespacedName]*apiv1.Service, error) {
	stream, err := readFileOrStdin(filename)
	if err != nil {
		return nil, err
	}

	unstructuredObjects, err := ExtractObjectsFromReader(bytes.NewReader(stream), namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	services := map[types.NamespacedName]*apiv1.Service{}
	for _, f := range unstructuredObjects {
		if !f.GroupVersionKind().Empty() && f.GroupVersionKind().Kind == "Service" {
			var service apiv1.Service
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &service)
			if err != nil {
				return nil, err
			}
			services[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}] = &service
		}

	}
	return services, nil
}

// ExtractObjectsFromReader extracts all objects from a reader,
// which is created from YAML or JSON input files.
// It retrieves all objects, including nested ones if they are contained within a list.
// The function takes a namespace parameter to optionally return only namespaced resources.
func ExtractObjectsFromReader(reader io.Reader, namespace string) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(reader, 4096)
	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := d.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		if u == nil {
			continue
		}
		if namespace != "" && u.GetNamespace() != namespace {
			continue
		}
		objs = append(objs, u)
	}

	finalObjs := []*unstructured.Unstructured{}
	for _, obj := range objs {
		tmpObjs := []*unstructured.Unstructured{}
		if obj.IsList() {
			err := obj.EachListItem(func(object runtime.Object) error {
				unstructuredObj, ok := object.(*unstructured.Unstructured)
				if ok {
					tmpObjs = append(tmpObjs, unstructuredObj)
					return nil
				}
				return fmt.Errorf("resource list item has unexpected type")
			})
			if err != nil {
				return nil, err
			}
		} else {
			tmpObjs = append(tmpObjs, obj)
		}
		finalObjs = append(finalObjs, tmpObjs...)
	}

	return finalObjs, nil
}
