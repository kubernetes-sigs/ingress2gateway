package nginx

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// genericReadFromCluster reads CRDs of type T from the cluster using the given GVK.
func genericReadFromCluster[T any](ctx context.Context, c client.Client, namespace string, gvk schema.GroupVersionKind, newObj func() *T) ([]T, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	if err := c.List(ctx, list); err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", gvk.GroupKind().String(), err)
	}

	var items []T
	for _, u := range list.Items {
		if namespace != "" && u.GetNamespace() != namespace {
			continue
		}
		obj := newObj()
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), obj); err != nil {
			return nil, fmt.Errorf("failed to parse %s object %s/%s: %w", gvk.Kind, u.GetNamespace(), u.GetName(), err)
		}
		items = append(items, *obj)
	}
	return items, nil
}

// genericReadFromFile reads CRDs of type T from a YAML file using the given GVK.
func genericReadFromFile[T any](filename string, namespace string, gvk schema.GroupVersionKind, newObj func() *T) ([]T, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", filename, err)
	}

	reader := bytes.NewReader(data)
	objs, err := common.ExtractObjectsFromReader(reader, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	var items []T
	for _, u := range objs {
		if namespace != "" && u.GetNamespace() != namespace {
			continue
		}
		if !u.GroupVersionKind().Empty() && u.GroupVersionKind() == gvk {
			obj := newObj()
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), obj); err != nil {
				return nil, fmt.Errorf("failed to parse %s object %s/%s: %w", gvk.Kind, u.GetNamespace(), u.GetName(), err)
			}
			items = append(items, *obj)
		}
	}
	return items, nil
}
