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
	"fmt"
	"os"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/logging"
	"github.com/spf13/cobra"
)

// kubeconfig indicates kubeconfig file location.
var kubeconfig string

// logFormat specifies the output format for logs (text or json).
var logFormat string

// logLevel specifies the minimum log level to output.
var logLevel string

// noColor disables ANSI color codes in text format log output.
var noColor bool

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ingress2gateway",
		Short: "Convert Ingress manifests to Gateway API manifests",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			getKubeconfig()
			initLogging()
		},
	}

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "",
		`The kubeconfig file to use when talking to the cluster. If the flag is not set, a set of standard locations can be searched for an existing kubeconfig file.`)

	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text",
		`Output format for logs. One of: text, json.`)

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		`Minimum log level to output. One of: debug, info, warn, error.`)

	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false,
		`Disable ANSI color codes in text format log output.`)

	return rootCmd
}

func initLogging() {
	cfg := logging.Config{
		Format:  logging.ParseFormat(logFormat),
		Level:   logging.ParseLevel(logLevel),
		Output:  os.Stderr,
		NoColor: noColor,
	}
	logging.Init(cfg)
}

func getKubeconfig() {
	if kubeconfig != "" {
		if err := os.Setenv("KUBECONFIG", kubeconfig); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set KUBECONFIG: %v\n", err)
			os.Exit(1)
		}
	}
}

func Execute() {
	rootCmd := newRootCmd()
	rootCmd.AddCommand(newPrintCommand())
	rootCmd.AddCommand(versionCmd)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
