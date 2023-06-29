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
package cmd

import (
	"os"
	"reflect"
	"testing"

	"k8s.io/cli-runtime/pkg/printers"
)

func Test_getResourcePrinter(t *testing.T) {
	testCases := []struct {
		name            string
		outputFormat    string
		expectedPrinter printers.ResourcePrinter
		expectingError  bool
	}{
		{
			name:            "JSON format",
			outputFormat:    "json",
			expectedPrinter: &printers.JSONPrinter{},
			expectingError:  false,
		},
		{
			name:            "YAML format",
			outputFormat:    "yaml",
			expectedPrinter: &printers.YAMLPrinter{},
			expectingError:  false,
		},
		{
			name:            "Default to YAML format",
			outputFormat:    "",
			expectedPrinter: &printers.YAMLPrinter{},
			expectingError:  false,
		},
		{
			name:            "Unsupported format",
			outputFormat:    "invalid",
			expectedPrinter: nil,
			expectingError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getResourcePrinter(tc.outputFormat)

			if tc.expectingError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectingError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}

			if !reflect.DeepEqual(result, tc.expectedPrinter) {
				t.Errorf("getResourcePrinter(%s) = %v, expected %v", tc.outputFormat, result, tc.expectedPrinter)
			}
		})

	}
}

func Test_getNamespaceFilter(t *testing.T) {
	testCases := []struct {
		name                      string
		namespace                 string
		allNamespaces             bool
		expectedNamespaceFilter   string
		expectingError            bool
		expectingCurrentNamespace bool
	}{
		{
			name:                      "Only namespace is specified",
			namespace:                 "default",
			allNamespaces:             false,
			expectedNamespaceFilter:   "default",
			expectingError:            false,
			expectingCurrentNamespace: false,
		},
		{
			name:                      "All namespaces overrides a specific namespace",
			namespace:                 "default",
			allNamespaces:             true,
			expectedNamespaceFilter:   "",
			expectingError:            false,
			expectingCurrentNamespace: false,
		},
		{
			name:                      "Current namespace used when nothing specified",
			namespace:                 "",
			allNamespaces:             false,
			expectedNamespaceFilter:   "_",
			expectingError:            false,
			expectingCurrentNamespace: true,
		},
	}

	destroy, err := setupKubeConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer destroy()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualNamespaceFilter, err := getNamespaceFilter(tc.namespace, tc.allNamespaces)

			if tc.expectingError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectingError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}

			if tc.expectingCurrentNamespace {
				tc.expectedNamespaceFilter, _ = getNamespaceFilter(tc.namespace, tc.allNamespaces)
			}

			if actualNamespaceFilter != tc.expectedNamespaceFilter {
				t.Errorf(`getNamespaceFilter("%s", %v) = %v, expected %v`,
					tc.namespace, tc.allNamespaces, actualNamespaceFilter, tc.expectedNamespaceFilter)
			}
		})

	}
}

func setupKubeConfig() (func(), error) {
	const kubeConfigFile = "/tmp/i2gw/.kube/config"

	if err := os.Setenv("KUBECONFIG", kubeConfigFile); err != nil {
		return nil, err
	}

	// Clean up from the last test, just in case...
	os.Remove(kubeConfigFile)

	content := []byte(`
apiVersion: v1
clusters:
- cluster:
    server: https://kubernetes.docker.internal:6443
  name: docker-desktop
- cluster:
    server: https://127.0.0.1:54873
  name: kind-i2gw
contexts:
- context:
    cluster: docker-desktop
    namespace: non-default-ns
    user: docker-desktop
  name: docker-desktop
- context:
    cluster: kind-i2gw
    user: kind-i2gw
  name: kind-i2gw
current-context: docker-desktop
kind: Config
preferences: {}
`)

	f, err := os.Create(kubeConfigFile)
	if err != nil {
		return nil, err
	}

	if _, err = f.Write(content); err != nil {
		os.Remove(kubeConfigFile)
		return nil, err
	}
	if err = f.Close(); err != nil {
		os.Remove(kubeConfigFile)
		return nil, err
	}

	return func() {
		os.Remove(kubeConfigFile)
	}, nil
}

func Test_getNamespaceInCurrentContext(t *testing.T) {
	destroy, err := setupKubeConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer destroy()

	expectedNamespace := "non-default-ns" // according to the kube-config at setupKubeConfig()
	actualNamespace, err := getNamespaceInCurrentContext()
	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	if expectedNamespace != actualNamespace {
		t.Errorf(`getNamespaceInCurrentContext() = "%s", %v, expected %s, %v`,
			actualNamespace, err, expectedNamespace, nil)
	}
}
