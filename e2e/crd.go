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

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	gatewayAPIVersion    = "v1.4.1"
	gatewayAPIInstallURL = "https://github.com/kubernetes-sigs/gateway-api/releases/download/" + gatewayAPIVersion + "/standard-install.yaml"
)

func deployCRDs(ctx context.Context, l logger, client *apiextensionsclientset.Clientset, skipCleanup bool) (func(), error) {
	l.Logf("Fetching manifests from %s", gatewayAPIInstallURL)
	yamlData, err := fetchManifestsWithRetry(ctx, l)
	if err != nil {
		return nil, fmt.Errorf("fetching manifests from %s: %w", gatewayAPIInstallURL, err)
	}

	crds, err := decodeCRDs(yamlData)
	if err != nil {
		return nil, fmt.Errorf("decoding CRDs: %w", err)
	}

	for _, crd := range crds {
		crd.TypeMeta = metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		}
		data, err := json.Marshal(crd)
		if err != nil {
			return nil, fmt.Errorf("converting CRD %s to JSON: %w", crd.Name, err)
		}

		// Use server-side apply.
		if _, err = client.ApiextensionsV1().CustomResourceDefinitions().Patch(ctx, crd.Name, types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: "ingress2gateway-e2e",
		}); err != nil {
			return nil, fmt.Errorf("applying CRD %s: %w", crd.Name, err)
		}
		l.Logf("Applied CRD %s", crd.Name)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of CRDs")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, crd := range crds {
			log.Printf("Deleting CRD %s", crd.Name)
			if err := client.ApiextensionsV1().CustomResourceDefinitions().Delete(cleanupCtx, crd.Name, metav1.DeleteOptions{}); err != nil {
				log.Printf("Deleting CRD %s: %v", crd.Name, err)
			}
		}
	}, nil
}

func decodeCRDs(yamlData []byte) ([]apiextensionsv1.CustomResourceDefinition, error) {
	objs, err := decodeManifests(yamlData)
	if err != nil {
		return nil, fmt.Errorf("decoding manifests: %w", err)
	}

	var out []apiextensionsv1.CustomResourceDefinition

	for _, obj := range objs {
		var crd apiextensionsv1.CustomResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &crd); err != nil {
			return nil, fmt.Errorf("converting object: %w", err)
		}

		if crd.Name == "" {
			continue
		}
		out = append(out, crd)
	}

	return out, nil
}

func fetchManifestsWithRetry(ctx context.Context, log logger) ([]byte, error) {
	var data []byte
	var err error
	const maxRetries = 5

	for i := range maxRetries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		data, err = fetchManifests(ctx)
		if err == nil {
			return data, nil
		}

		log.Logf("Fetching manifests (attempt %d/%d): %v", i+1, maxRetries, err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return nil, err
}

func fetchManifests(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gatewayAPIInstallURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting manifests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response data: %w", err)
	}

	return data, nil
}

func decodeManifests(data []byte) ([]unstructured.Unstructured, error) {
	var out []unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)

	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding object: %w", err)
		}
		if obj.Object == nil {
			continue
		}
		out = append(out, obj)
	}

	return out, nil
}
