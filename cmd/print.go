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
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

var (
	// outputFormat contains currently set output format. Value assigned via --output/-o flag.
	// Defaults to YAML.
	outputFormat = "yaml"
	// The path to the input yaml config file. Value assigned via --input_file/-i flag
	inputFile = ""
)

// printCmd represents the print command. It prints HTTPRoutes and Gateways
// generated from Ingress resources.
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Prints HTTPRoutes and Gateways generated from Ingress resources",
	RunE: func(cmd *cobra.Command, args []string) error {
		resourcePrinter, err := getResourcePrinter(outputFormat)
		if err != nil {
			return err
		}
		i2gw.Run(resourcePrinter, inputFile)
		return nil
	},
}

// getResourcePrinter returns a specific type of printers.ResourcePrinter
// based on the provided outputFormat.
func getResourcePrinter(outputFormat string) (printers.ResourcePrinter, error) {
	switch outputFormat {
	case "yaml", "":
		return &printers.YAMLPrinter{}, nil
	case "json":
		return &printers.JSONPrinter{}, nil
	default:
		return nil, fmt.Errorf("%s is not a supported output format", outputFormat)
	}
}

func init() {
	var printFlags genericclioptions.JSONYamlPrintFlags
	allowedFormats := printFlags.AllowedFormats()

	printCmd.Flags().StringVarP(&outputFormat, "output", "o", "yaml",
		fmt.Sprintf(`Output format. One of: (%s)`, strings.Join(allowedFormats, ", ")))

	printCmd.Flags().StringVarP(&inputFile, "input_file", "i", "",
		fmt.Sprintf(`Path to your input yaml file. Default to ingress resources in your kubernetes cluster`))

	rootCmd.AddCommand(printCmd)
}
