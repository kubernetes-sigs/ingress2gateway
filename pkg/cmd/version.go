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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/cmd/version"
)

const (
	yamlOutput = "yaml"
	jsonOutput = "json"
)

var (
	output string
)

func RegisterVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Show versions.",
		Long:    "",
		RunE: func(cmd *cobra.Command, args []string) error {
			return version.Print(cmd.OutOrStdout(), output)
		},
	}

	cmd.PersistentFlags().StringVarP(&output, "output", "o", yamlOutput, "One of 'yaml' or 'json'")

	return cmd
}
