/*
Copyright © 2022 Kubernetes Authors

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
	"os"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

type rootCmdOptions struct {
	OutputFormat string
}

func newRootCmdOptions() *rootCmdOptions {
	return &rootCmdOptions{
		OutputFormat: "yaml",
	}
}

func newRootRunner(options *rootCmdOptions) *i2gw.RootRunner {
	runner := i2gw.RootRunner{
		ResourcePrinter: &printers.YAMLPrinter{},
	}
	if options.OutputFormat == "json" {
		runner.ResourcePrinter = &printers.JSONPrinter{}
	}
	return &runner
}

func newRootCmd() *cobra.Command {
	o := newRootCmdOptions()

	cmd := cobra.Command{
		Use:   "ingress2gateway",
		Short: "Convert Ingress manifests to Gateway API manifests",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.ParseFlags(args); err != nil {
				fmt.Printf("Error parsing flags: %v", err)
			}

			newRootRunner(o).Run()
		},
	}

	var printFlags genericclioptions.JSONYamlPrintFlags
	allowedFormats := printFlags.AllowedFormats()

	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", o.OutputFormat,
		fmt.Sprintf(`Output format. One of: (%s)`, strings.Join(allowedFormats, ", ")))

	return &cmd
}

func Execute() {
	err := newRootCmd().Execute()
	if err != nil {
		os.Exit(1)
	}
}
