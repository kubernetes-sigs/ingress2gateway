/*
Copyright 2026 The Kubernetes Authors.

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

package traefik

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	standard_emitter "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/standard"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const fixturesDir = "./fixtures"

func TestFileConversion(t *testing.T) {
	ctx := context.Background()

	filepath.WalkDir(filepath.Join(fixturesDir, "input"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err.Error())
		}
		if d.IsDir() {
			return nil
		}

		t.Run(d.Name(), func(t *testing.T) {
			traefikProvider := NewProvider(&i2gw.ProviderConf{})

			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				t.Fatalf("failed to read input file %v: %v", d.Name(), err)
			}

			if err := traefikProvider.ReadResourcesFromFile(ctx, bytes.NewReader(data)); err != nil {
				t.Fatalf("failed to read resources from file %v: %v", d.Name(), err)
			}

			ir, errList := traefikProvider.ToIR()
			if len(errList) > 0 {
				t.Fatalf("unexpected errors converting %v to IR: %v", d.Name(), errList.ToAggregate().Error())
			}

			emitter := standard_emitter.NewEmitter(&i2gw.EmitterConf{})
			gotResources, errList := emitter.Emit(ir)
			if len(errList) > 0 {
				t.Fatalf("unexpected errors emitting %v: %v", d.Name(), errList.ToAggregate().Error())
			}

			outputFile := filepath.Join(fixturesDir, "output", d.Name())
			wantResources, err := readGatewayResourcesFromFile(t, outputFile)
			if err != nil {
				t.Fatalf("failed to read expected output from %v: %v", outputFile, err)
			}

			if !apiequality.Semantic.DeepEqual(gotResources.Gateways, wantResources.Gateways) {
				t.Errorf("Gateways mismatch for %v (-want +got):\n%s",
					d.Name(), cmp.Diff(wantResources.Gateways, gotResources.Gateways))
			}

			if !apiequality.Semantic.DeepEqual(gotResources.HTTPRoutes, wantResources.HTTPRoutes) {
				t.Errorf("HTTPRoutes mismatch for %v (-want +got):\n%s",
					d.Name(), cmp.Diff(wantResources.HTTPRoutes, gotResources.HTTPRoutes))
			}
		})

		return nil
	})
}

func readGatewayResourcesFromFile(t *testing.T, filename string) (*i2gw.GatewayResources, error) {
	t.Helper()

	stream, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", filename, err)
	}

	unstructuredObjects, err := common.ExtractObjectsFromReader(bytes.NewReader(stream), "")
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	res := i2gw.GatewayResources{
		Gateways:   make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes: make(map[types.NamespacedName]gatewayv1.HTTPRoute),
	}

	for _, obj := range unstructuredObjects {
		switch obj.GetKind() {
		case "Gateway":
			var gw gatewayv1.Gateway
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &gw); err != nil {
				return nil, fmt.Errorf("failed to parse Gateway object: %w", err)
			}
			res.Gateways[types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name}] = gw
		case "HTTPRoute":
			var httpRoute gatewayv1.HTTPRoute
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &httpRoute); err != nil {
				return nil, fmt.Errorf("failed to parse HTTPRoute object: %w", err)
			}
			res.HTTPRoutes[types.NamespacedName{Namespace: httpRoute.Namespace, Name: httpRoute.Name}] = httpRoute
		default:
			return nil, fmt.Errorf("unexpected object kind %q in fixture %v", obj.GetKind(), filename)
		}
	}

	return &res, nil
}
