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
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// outputFormat contains currently set output format. Value assigned via --output/-o flag.
	// Defaults to YAML.
	outputFormat = "yaml"

	// The path to the input yaml config file. Value assigned via --input_file flag
	inputFile string

	// The namespace used to query Gateway API objects. Value assigned via
	// --namespace/-n flag.
	// On absence, the current user active namespace is used.
	namespace string

	// allNamespaces indicates whether all namespaces should be used. Value assigned via
	// --all-namespaces/-A flag.
	allNamespaces bool
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
		namespaceFilter, err := getNamespaceFilter(namespace, allNamespaces)
		if err != nil && inputFile == "" {
			// When asked to read from the cluster, but getting the current namespace
			// failed for whatever reason - do not process the request.
			return err
		}
		// If err is nil we got the right filtered namespace.
		// If the input file is specified, and we failed to get the namespace, use all namespaces.
		i2gw.Run(resourcePrinter, namespaceFilter, inputFile)
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

// getNamespaceFilter returns a namespace filter, taking into consideration whether a specific
// namespace is requested, or all of them are.
func getNamespaceFilter(requestedNamespace string, useAllNamespaces bool) (string, error) {

	// When we should use all namespaces, return an empty string.
	// This is the first condition since it should override the requestedNamespace,
	// if specified.
	if useAllNamespaces {
		return "", nil
	}

	if requestedNamespace == "" {
		return getNamespaceInCurrentContext()
	}
	return requestedNamespace, nil
}

// getNamespaceInCurrentContext returns the namespace in the current active context of the user.
func getNamespaceInCurrentContext() (string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	currentNamespace, _, err := kubeConfig.Namespace()

	return currentNamespace, err
}

func init() {
	var printFlags genericclioptions.JSONYamlPrintFlags
	allowedFormats := printFlags.AllowedFormats()

	printCmd.Flags().StringVarP(&outputFormat, "output", "o", "yaml",
		fmt.Sprintf(`Output format. One of: (%s)`, strings.Join(allowedFormats, ", ")))

	printCmd.Flags().StringVar(&inputFile, "input_file", "",
		`Path to the manifest file. When set, the tool will read ingresses from the file instead of reading from the cluster. Supported files are yaml and json`)

	printCmd.Flags().StringVarP(&namespace, "namespace", "n", "",
		`If present, the namespace scope for this CLI request`)

	printCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false,
		`If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even
if specified with --namespace.`)

	printCmd.MarkFlagsMutuallyExclusive("namespace", "all-namespaces")

	rootCmd.AddCommand(printCmd)
}
