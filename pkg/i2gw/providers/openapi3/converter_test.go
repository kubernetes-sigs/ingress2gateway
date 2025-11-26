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

package openapi3

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

const fixturesDir = "./fixtures"

func TestFileConvertion(t *testing.T) {
	ctx := context.Background()

	type testData struct {
		providerConf          *i2gw.ProviderConf
		expectedReadFileError error
	}

	defaultTestData := testData{
		providerConf: &i2gw.ProviderConf{
			ProviderSpecificFlags: map[string]map[string]string{
				"openapi3": {
					"gateway-class-name": "external",
					"gateway-tls-secret": "gateway-tls-cert",
					"backend":            "backend-1:3000",
				},
			},
		},
	}

	customTestData := map[string]testData{
		"reference-grants.yaml": {
			providerConf: &i2gw.ProviderConf{
				Namespace: "networking",
				ProviderSpecificFlags: map[string]map[string]string{
					"openapi3": {
						"gateway-class-name": "external",
						"gateway-tls-secret": "secrets/gateway-tls-cert",
						"backend":            "apps/backend-1",
					},
				},
			},
		},
		"invalid-spec.yaml": {
			expectedReadFileError: fmt.Errorf("failed to read resources from file: invalid OpenAPI 3.x spec"),
		},
	}

	filepath.WalkDir(filepath.Join(fixturesDir, "input"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err.Error())
		}
		if d.IsDir() {
			return nil
		}

		providerConf := defaultTestData.providerConf
		expectedReadFileError := defaultTestData.expectedReadFileError

		inputFileName := regexp.MustCompile(`\d+-(.+\.(json|yaml))$`).FindAllStringSubmatch(d.Name(), -1)[0][1]
		data, ok := customTestData[inputFileName]
		if ok {
			if data.providerConf != nil {
				providerConf = data.providerConf
			}
			if data.expectedReadFileError != nil {
				expectedReadFileError = data.expectedReadFileError
			}
		}

		provider := NewProvider(providerConf)

		if readFileErr := provider.ReadResourcesFromFile(ctx, path); readFileErr != nil {
			if expectedReadFileError == nil {
				t.Fatalf("unexpected error during reading test file %v: %v", d.Name(), readFileErr.Error())
			} else if !strings.Contains(readFileErr.Error(), expectedReadFileError.Error()) {
				t.Fatalf("unexpected error during reading test file %v: '%v' does not contain expected '%v'", d.Name(), readFileErr.Error(), expectedReadFileError.Error())
			} else {
				return nil // success
			}
		} else if expectedReadFileError != nil {
			t.Fatalf("missing expected error during reading test file %v: %v", d.Name(), expectedReadFileError.Error())
		}

		gotIR, errList := provider.ToIR()
		if len(errList) > 0 {
			t.Fatalf("unexpected errors during input conversion to ir for file %v: %v", d.Name(), errList.ToAggregate().Error())
		}

		outputFile := filepath.Join(fixturesDir, "output", d.Name())
		wantIR, err := readGatewayResourcesFromFile(t, outputFile)
		if err != nil {
			t.Fatalf("failed to read wantIR from file %v: %v", outputFile, err.Error())
		}

		if !apiequality.Semantic.DeepEqual(gotIR.Gateways, wantIR.Gateways) {
			t.Errorf("Gateways diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantIR.Gateways, gotIR.Gateways))
		}

		if !apiequality.Semantic.DeepEqual(gotIR.HTTPRoutes, wantIR.HTTPRoutes) {
			t.Errorf("HTTPRoutes diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantIR.HTTPRoutes, gotIR.HTTPRoutes))
		}

		if !apiequality.Semantic.DeepEqual(gotIR.TLSRoutes, wantIR.TLSRoutes) {
			t.Errorf("TLSRoutes diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantIR.TLSRoutes, gotIR.TLSRoutes))
		}

		if !apiequality.Semantic.DeepEqual(gotIR.TCPRoutes, wantIR.TCPRoutes) {
			t.Errorf("TCPRoutes diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantIR.TCPRoutes, gotIR.TCPRoutes))
		}

		if !apiequality.Semantic.DeepEqual(gotIR.ReferenceGrants, wantIR.ReferenceGrants) {
			t.Errorf("ReferenceGrants diff for file %v (-want +got): %s", d.Name(), cmp.Diff(wantIR.ReferenceGrants, gotIR.ReferenceGrants))
		}

		return nil
	})
}

func readGatewayResourcesFromFile(t *testing.T, filename string) (*provider_intermediate.IR, error) {
	t.Helper()

	stream, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %v: %w", filename, err)
	}

	unstructuredObjects, err := common.ExtractObjectsFromReader(bytes.NewReader(stream), "")
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}

	res := provider_intermediate.IR{
		Gateways:        make(map[types.NamespacedName]provider_intermediate.GatewayContext),
		HTTPRoutes:      make(map[types.NamespacedName]provider_intermediate.HTTPRouteContext),
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
			}] = provider_intermediate.GatewayContext{Gateway: gw}
		case "HTTPRoute":
			var httpRoute gatewayv1.HTTPRoute
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &httpRoute); err != nil {
				return nil, fmt.Errorf("failed to parse k8s gateway HTTPRoute object: %w", err)
			}

			res.HTTPRoutes[types.NamespacedName{
				Namespace: httpRoute.Namespace,
				Name:      httpRoute.Name,
			}] = provider_intermediate.HTTPRouteContext{HTTPRoute: httpRoute}
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
