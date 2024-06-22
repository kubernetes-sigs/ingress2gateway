/*
Copyright 2022 The Kubernetes Authors.

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

	"github.com/spf13/cobra"
)

// kubeconfig indicates kubeconfig file location.
var kubeconfig string

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ingress2gateway",
		Short: "Convert Ingress manifests to Gateway API manifests",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			getKubeconfig()
		},
	}

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "",
		`The kubeconfig file to use when talking to the cluster. If the flag is not set, a set of standard locations can be searched for an existing kubeconfig file.`)
	return rootCmd
}

func getKubeconfig() {
	if kubeconfig != "" {
		os.Setenv("KUBECONFIG", kubeconfig)
	}
}

func Execute() {
	rootCmd := newRootCmd()
	rootCmd.AddCommand(newPrintCommand())
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
