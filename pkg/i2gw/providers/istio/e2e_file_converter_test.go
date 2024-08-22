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
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const fixturesDir = "./fixtures"

func TestFileConversion(t *testing.T) {
	ctx := context.Background()

	filepath.WalkDir(filepath.Join(fixturesDir, "input"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatalf(err.Error())
		}
		if d.IsDir() {
			return nil
		}

		istioProvider := NewProvider(&i2gw.ProviderConf{})

		err = istioProvider.ReadResourcesFromFile(ctx, path)
		if err != nil {
			t.Fatalf("Failed to read input from file %v: %v", d.Name(), err.Error())
		}

		ir, errList := istioProvider.ToIR()
		if len(errList) > 0 {
			t.Fatalf("unexpected errors during input conversion to ir for file %v: %v", d.Name(), errList.ToAggregate().Error())
		}
		gotGatewayResources, errList := istioProvider.ToGatewayResources(ir)
		if len(errList) > 0 {
			t.Fatalf("unexpected errors during ir conversion to Gateway for file %v: %v", d.Name(), errList.ToAggregate().Error())
		}

		outputFile := filepath.Join(fixturesDir, "output", d.Name())
		wantGatewayResources, err := readGatewayResourcesFromFile(t, outputFile)
		if err != nil {
			t.Fatalf("failed to read wantGatewayResources from file %v: %v", outputFile, err.Error())
		}

		if !apiequality.Semantic.DeepEqual(gotGatewayResources.Gateways, wantGatewayResources.Gateways) {
			t.Errorf("Gateways diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantGatewayResources.Gateways, gotGatewayResources.Gateways))
		}

		if !apiequality.Semantic.DeepEqual(gotGatewayResources.HTTPRoutes, wantGatewayResources.HTTPRoutes) {
			t.Errorf("HTTPRoutes diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantGatewayResources.HTTPRoutes, gotGatewayResources.HTTPRoutes))
		}

		if !apiequality.Semantic.DeepEqual(gotGatewayResources.TLSRoutes, wantGatewayResources.TLSRoutes) {
			t.Errorf("TLSRoutes diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantGatewayResources.TLSRoutes, gotGatewayResources.TLSRoutes))
		}

		if !apiequality.Semantic.DeepEqual(gotGatewayResources.TCPRoutes, wantGatewayResources.TCPRoutes) {
			t.Errorf("TCPRoutes diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantGatewayResources.TCPRoutes, gotGatewayResources.TCPRoutes))
		}

		if !apiequality.Semantic.DeepEqual(gotGatewayResources.ReferenceGrants, wantGatewayResources.ReferenceGrants) {
			t.Errorf("ReferenceGrants diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantGatewayResources.ReferenceGrants, gotGatewayResources.ReferenceGrants))
		}

		return nil
	})
}

func readGatewayResourcesFromFile(t *testing.T, filename string) (*i2gw.GatewayResources, error) {
	t.Helper()

	stream, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", filename, err)
	}

	unstructuredObjects, err := common.ExtractObjectsFromReader(bytes.NewReader(stream), "")
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	res := i2gw.GatewayResources{
		Gateways:        make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes:      make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		TLSRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		ReferenceGrants: make(map[types.NamespacedName]gatewayv1beta1.ReferenceGrant),
	}

	for _, obj := range unstructuredObjects {
		switch objKind := obj.GetKind(); objKind {
		case "Gateway":
			var gw gatewayv1.Gateway
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &gw); err != nil {
				return nil, fmt.Errorf("failed to parse k8s gateway object: %w", err)
			}
			res.Gateways[types.NamespacedName{
				Namespace: gw.Namespace,
				Name:      gw.Name,
			}] = gw
		case "HTTPRoute":
			var httpRoute gatewayv1.HTTPRoute
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &httpRoute); err != nil {
				return nil, fmt.Errorf("failed to parse k8s gateway HTTPRoute object: %w", err)
			}

			res.HTTPRoutes[types.NamespacedName{
				Namespace: httpRoute.Namespace,
				Name:      httpRoute.Name,
			}] = httpRoute
		case "TLSRoute":
			var tlsRoute gatewayv1alpha2.TLSRoute
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &tlsRoute); err != nil {
				return nil, fmt.Errorf("failed to parse k8s gateway TLSRoute object: %w", err)
			}

			res.TLSRoutes[types.NamespacedName{
				Namespace: tlsRoute.Namespace,
				Name:      tlsRoute.Name,
			}] = tlsRoute
		case "TCPRoute":
			var tcpRoute gatewayv1alpha2.TCPRoute
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &tcpRoute); err != nil {
				return nil, fmt.Errorf("failed to parse k8s gateway TCPRoute object: %w", err)
			}

			res.TCPRoutes[types.NamespacedName{
				Namespace: tcpRoute.Namespace,
				Name:      tcpRoute.Name,
			}] = tcpRoute
		case "ReferenceGrant":
			var referenceGrant gatewayv1beta1.ReferenceGrant
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &referenceGrant); err != nil {
				return nil, fmt.Errorf("failed to parse k8s gateway ReferenceGrant object: %w", err)
			}

			res.ReferenceGrants[types.NamespacedName{
				Namespace: referenceGrant.Namespace,
				Name:      referenceGrant.Name,
			}] = referenceGrant
		default:
			return nil, fmt.Errorf("unknown object kind: %v", objKind)
		}
	}

	return &res, nil
}
