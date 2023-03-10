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
	"github.com/spf13/cobra"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/translator"
)

func RegisterTranslateCommand() *cobra.Command {
	var (
		mode         string
		file         string
		resourceType string
		output       string
		provider     string
	)

	cmd := &cobra.Command{
		Use:   "translate",
		Short: "Translates Ingress resources into Gateway API resources.",
		Example: `  # Translate Ingress Resources into Gateway API Resources Locally.
  i2gw translate --file <input file> --mode local

  # Translate Ingress Resources into All supported Gateway API resources from remote cluster in JSON output.
  i2gw translate --type all --output json --mode remote

  # Translate Ingress Resources into All supported Gateway API resources from remote cluster in YAML output.
  i2gw translate --type all --output yaml --mode remote

  # Translate Ingress Resources into All supported Gateway API resources in JSON output.
  i2gw translate --type all --output json --file <input file> --mode local

  # Translate Ingress Resources into All supported Gateway API resources in YAML output.
  i2gw translate --type all --output yaml --file <input file> --mode local

  # Translate Ingress Resources into Gateway Gateway API resources.
  i2gw translate --type gateway --file <input file> --mode local

  # Translate Ingress Resources into HTTPRoute Gateway API resources.
  i2gw translate --type httproute --file <input file> --mode local

  # Translate Ingress Resources into HTTPRoute Gateway API resources with short syntax.
  i2gw translate -t httproute -o yaml -f <input file> -m local
	  `,
		RunE: func(cmd *cobra.Command, args []string) error {
			options := translator.NewOptions(
				cmd.OutOrStdout(),
				mode,
				file,
				output,
				resourceType,
				provider,
			)
			translator := translator.New(options)

			return translator.Run()
		},
	}

	cmd.PersistentFlags().StringVarP(&mode, "mode", "m", translator.RemoteMode, "If retrieves Ingress resources from local or cluster")
	cmd.PersistentFlags().StringVarP(&file, "file", "f", "", "Location of input file, only used when mode is local")
	cmd.PersistentFlags().StringVarP(&output, "output", "o", yamlOutput, "One of 'yaml' or 'json'")
	cmd.PersistentFlags().StringVarP(&provider, "provider", "p", string(translator.IngressNginxIngressProvider), "Specify the Ingress Provider")
	cmd.PersistentFlags().StringVarP(&resourceType, "type", "t", "all", translator.GetValidResourceTypesStr())

	return cmd
}
