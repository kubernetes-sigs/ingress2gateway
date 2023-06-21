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
	output = "yaml"
)

// printCmd represents the print command. It prints ingress-converted
// HTTPRoutes and Gateways.
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Prints ingress-converted HTTPRoutes and Gateways",
	Run: func(cmd *cobra.Command, args []string) {
		resourcePrinter := getResourcePrinter()
		i2gw.Run(resourcePrinter)
	},
}

func getResourcePrinter() printers.ResourcePrinter {
	if output == "json" {
		return &printers.JSONPrinter{}
	}
	return &printers.YAMLPrinter{}
}

func init() {
	var printFlags genericclioptions.JSONYamlPrintFlags
	allowedFormats := printFlags.AllowedFormats()

	printCmd.Flags().StringVarP(&output, "output", "o", "yaml",
		fmt.Sprintf(`Output format. One of: (%s)`, strings.Join(allowedFormats, ", ")))

	rootCmd.AddCommand(printCmd)
}
