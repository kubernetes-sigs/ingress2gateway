/*
Copyright © 2023 Kubernetes Authors

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

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/translator"
)

func TestTranslate(t *testing.T) {
	testCases := []struct {
		name         string
		output       string
		resourceType translator.GWAPIResourceType
		errorCount   int
	}{
		{
			name:       "from-ingress-to-simple-gwapi",
			output:     jsonOutput,
			errorCount: 0,
		},
		{
			name:       "from-ingress-to-simple-gwapi",
			output:     yamlOutput,
			errorCount: 0,
		},
		{
			name:       "from-ingress-to-simple-gwapi",
			errorCount: 0,
		},
		{
			name:         "from-ingress-to-simple-gwapi",
			output:       yamlOutput,
			resourceType: "unknown",
			errorCount:   1,
		},
		{
			name:         "from-ingress-to-simple-gwapi",
			output:       yamlOutput,
			resourceType: translator.AllGWAPIResourceType,
			errorCount:   0,
		},
		{
			name:         "from-ingress-to-simple-gwapi",
			output:       yamlOutput,
			resourceType: translator.HTTPRouteGWAPIResourceType,
			errorCount:   0,
		},
		{
			name:         "from-ingress-to-simple-gwapi",
			output:       yamlOutput,
			resourceType: translator.GatewayGWAPIResourceType,
			errorCount:   0,
		}, {
			name:         "from-ingress-to-simple-gwapi",
			output:       jsonOutput,
			resourceType: translator.HTTPRouteGWAPIResourceType,
			errorCount:   0,
		},
		{
			name:         "from-ingress-to-simple-gwapi",
			output:       jsonOutput,
			resourceType: translator.GatewayGWAPIResourceType,
			errorCount:   0,
		}, {
			name:         "from-ingress-to-canary-gwapi",
			output:       yamlOutput,
			resourceType: translator.AllGWAPIResourceType,
			errorCount:   0,
		}, {
			name:         "from-ingress-to-canary-gwapi",
			output:       jsonOutput,
			resourceType: translator.AllGWAPIResourceType,
			errorCount:   0,
		}, {
			name:         "from-ingress-to-multi-gateways-gwapi",
			output:       yamlOutput,
			resourceType: translator.AllGWAPIResourceType,
			errorCount:   0,
		}, {
			name:         "from-ingress-to-multi-gateways-gwapi",
			output:       yamlOutput,
			resourceType: translator.GatewayGWAPIResourceType,
			errorCount:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s in %s type with %s output", tc.name, tc.resourceType, tc.output), func(t *testing.T) {
			b := bytes.NewBufferString("")
			root := RegisterTranslateCommand()
			root.SetOut(b)
			root.SetErr(b)
			args := []string{
				"translate",
				"--mode",
				translator.LocalMode,
				"--file",
				"testdata/in/" + tc.name + ".yaml",
			}

			var resourceType translator.GWAPIResourceType
			if tc.resourceType == "" {
				resourceType = translator.AllGWAPIResourceType
			} else {
				resourceType = tc.resourceType
				args = append(args, "--type", string(tc.resourceType))
			}

			if tc.output == yamlOutput {
				args = append(args, "--output", yamlOutput)
			} else if tc.output == jsonOutput {
				args = append(args, "--output", jsonOutput)
			}

			root.SetArgs(args)

			if tc.errorCount == 0 {
				assert.NoError(t, root.ExecuteContext(context.Background()))
			} else {
				assert.Error(t, root.ExecuteContext(context.Background()))
				return
			}

			out, err := io.ReadAll(b)
			assert.NoError(t, err)

			if tc.output == jsonOutput {
				require.JSONEq(t, requireTestDataOutFile(t, tc.name+"."+string(resourceType)+".json"), string(out))
			} else {
				require.YAMLEq(t, requireTestDataOutFile(t, tc.name+"."+string(resourceType)+".yaml"), string(out))
			}
		})
	}
}

func requireTestDataOutFile(t *testing.T, name ...string) string {
	t.Helper()
	elems := append([]string{"testdata", "out"}, name...)
	content, err := os.ReadFile(filepath.Join(elems...))
	require.NoError(t, err)
	return string(content)
}
