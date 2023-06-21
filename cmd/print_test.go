package cmd

import (
	"k8s.io/cli-runtime/pkg/printers"
	"reflect"
	"testing"
)

func Test_getResourcePrinter(t *testing.T) {
	testCases := []struct {
		outputFormat    string
		expectedPrinter printers.ResourcePrinter
	}{
		{
			outputFormat:    "json",
			expectedPrinter: &printers.JSONPrinter{},
		},
		{
			outputFormat:    "yaml",
			expectedPrinter: &printers.YAMLPrinter{},
		},
		{
			outputFormat:    "",
			expectedPrinter: &printers.YAMLPrinter{},
		},
		{
			outputFormat:    "invalid",
			expectedPrinter: nil,
		},
	}

	for _, tc := range testCases {
		result := getResourcePrinter(tc.outputFormat)

		if !reflect.DeepEqual(result, tc.expectedPrinter) {
			t.Errorf("getResourcePrinter(%s) = %v, expected %v", tc.outputFormat, result, tc.expectedPrinter)
		}
	}
}
