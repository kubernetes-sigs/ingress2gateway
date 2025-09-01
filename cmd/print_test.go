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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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
			pr := PrintRunner{outputFormat: tc.outputFormat}
			err := pr.initializeResourcePrinter()

			if tc.expectingError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectingError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}

			if !reflect.DeepEqual(pr.resourcePrinter, tc.expectedPrinter) {
				t.Errorf("getResourcePrinter(%s) = %v, expected %v", tc.outputFormat, pr.resourcePrinter, tc.expectedPrinter)
			}
		})

	}
}

func Test_initializeParentRefs(t *testing.T) {
	testCases := []struct {
		name               string
		parentRefs         []string
		expectedParentRefs []gatewayv1.ParentReference
		expectedError      string
	}{
		{
			name:               "empty parentref should return null parentRefs",
			expectedParentRefs: nil,
		},
		{
			name:       "resource only parentRef should return just the object name set",
			parentRefs: []string{"someresource"},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name: "someresource",
				},
			},
		},
		{
			name:       "resource and namespace parentRef should return the right object",
			parentRefs: []string{"somens/someresource"},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "someresource",
					Namespace: ptr.To(gatewayv1.Namespace("somens")),
				},
			},
		},
		{
			name:       "resource, namespace and kind parentRef should return the right object",
			parentRefs: []string{"somens/someresource=Something"},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "someresource",
					Namespace: ptr.To(gatewayv1.Namespace("somens")),
					Kind:      ptr.To(gatewayv1.Kind("Something")),
				},
			},
		},
		{
			name:       "resource, namespace, group kind parentRef should return the right object",
			parentRefs: []string{"somens/someresource=somegroup.k8s.io/Something"},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "someresource",
					Namespace: ptr.To(gatewayv1.Namespace("somens")),
					Kind:      ptr.To(gatewayv1.Kind("Something")),
					Group:     ptr.To(gatewayv1.Group("somegroup.k8s.io")),
				},
			},
		},
		{
			name:       "resource, namespace and sectionname parentRef should return the right object",
			parentRefs: []string{"somens/someresource:section1"},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:        "someresource",
					Namespace:   ptr.To(gatewayv1.Namespace("somens")),
					SectionName: ptr.To(gatewayv1.SectionName("section1")),
				},
			},
		},
		{
			name:       "resource, namespace and port parentRef should return the right object",
			parentRefs: []string{"somens/someresource::12345"},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "someresource",
					Namespace: ptr.To(gatewayv1.Namespace("somens")),
					Port:      ptr.To(gatewayv1.PortNumber(12345)),
				},
			},
		},
		{
			name:       "resource, namespace, sectionname, port and resource kind parentRef should return the right object",
			parentRefs: []string{"somens/someresource:section1:12345=Gateway"},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:        "someresource",
					Namespace:   ptr.To(gatewayv1.Namespace("somens")),
					SectionName: ptr.To(gatewayv1.SectionName("section1")),
					Kind:        ptr.To(gatewayv1.Kind("Gateway")),
					Port:        ptr.To(gatewayv1.PortNumber(12345)),
				},
			},
		},
		{
			name:          "invalid port should return an error",
			parentRefs:    []string{"somens/someresource:section1:xpto=Gateway"},
			expectedError: "Gateway contains invalid port number",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pr := PrintRunner{
				parentRefs: tc.parentRefs,
			}
			err := pr.initializeParentRefs()

			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("got invalid error, expecting=%s got=%s", tc.expectedError, err.Error())
				}
			}

			if tc.expectedError == "" && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}

			if tc.expectedError == "" && !reflect.DeepEqual(tc.expectedParentRefs, pr.parsedParentRefs) {
				t.Errorf("parsedParentRef does not match. Expected=%s got=%s", spew.Sdump(tc.expectedParentRefs), spew.Sdump(pr.parsedParentRefs))
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
			pr := PrintRunner{
				namespace:     tc.namespace,
				allNamespaces: tc.allNamespaces,
			}
			err = pr.initializeNamespaceFilter()

			if tc.expectingError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectingError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}

			if tc.expectingCurrentNamespace {
				tc.expectedNamespaceFilter = pr.namespaceFilter
			}

			if pr.namespaceFilter != tc.expectedNamespaceFilter {
				t.Errorf(`getNamespaceFilter("%s", %v) = %v, expected %v`,
					tc.namespace, tc.allNamespaces, pr.namespaceFilter, tc.expectedNamespaceFilter)
			}
		})

	}
}

func setupKubeConfig() (func(), error) {

	// Clean up from the last test, just in case...
	cleanupFunc := func() {
		globPattern := filepath.Join(os.TempDir(), "*-kube")
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			log.Fatalf("Failed to match %q: %v", globPattern, err)
		}
		for _, match := range matches {
			if err = os.RemoveAll(match); err != nil {
				log.Printf("Failed to remove %q: %v", match, err)
			}
		}
	}
	cleanupFunc()

	content := []byte(`
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: example
- cluster:
    server: https://127.0.0.1:54873
  name: kind-i2gw
contexts:
- context:
    cluster: example
    namespace: non-default-ns
    user: example
  name: example
- context:
    cluster: kind-i2gw
    user: kind-i2gw
  name: kind-i2gw
current-context: example
kind: Config
preferences: {}
`)

	dir, err := os.MkdirTemp(os.TempDir(), "*-kube")
	if err != nil {
		log.Println(err)
	}

	kubeConfigFile := fmt.Sprintf("%s/config", dir)

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

	if err = os.Setenv("KUBECONFIG", kubeConfigFile); err != nil {
		return nil, err
	}

	return cleanupFunc, nil
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

func Test_getProviderSpecificFlags(t *testing.T) {
	value1 := "value1"
	value2 := "value2"
	testCases := []struct {
		name                  string
		providerSpecificFlags map[string]*string
		providers             []string
		expected              map[string]map[string]string
	}{
		{
			name:                  "No provider specific configuration",
			providerSpecificFlags: make(map[string]*string),
			providers:             []string{"provider"},
			expected:              map[string]map[string]string{},
		},
		{
			name:                  "Provider specific configuration matching provider in the list",
			providerSpecificFlags: map[string]*string{"provider-conf": &value1},
			providers:             []string{"provider"},
			expected: map[string]map[string]string{
				"provider": {"conf": value1},
			},
		},
		{
			name: "Provider specific configuration matching providers in the list with multiple providers",
			providerSpecificFlags: map[string]*string{
				"provider-a-conf1": &value1,
				"provider-b-conf2": &value2,
			},
			providers: []string{"provider-a", "provider-b", "provider-c"},
			expected: map[string]map[string]string{
				"provider-a": {"conf1": value1},
				"provider-b": {"conf2": value2},
			},
		},
		{
			name:                  "Provider specific configuration not matching provider in the list",
			providerSpecificFlags: map[string]*string{"provider-conf": &value1},
			providers:             []string{"provider-a", "provider-b", "provider-c"},
			expected:              map[string]map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pr := PrintRunner{
				providerSpecificFlags: tc.providerSpecificFlags,
				providers:             tc.providers,
			}
			actual := pr.getProviderSpecificFlags()
			if diff := cmp.Diff(tc.expected, actual); diff != "" {
				t.Errorf("Unexpected provider-specific flags, \n want: %+v\n got: %+v\n diff (-want +got):\n%s", tc.expected, actual, diff)
			}
		})
	}
}
