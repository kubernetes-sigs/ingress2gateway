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
