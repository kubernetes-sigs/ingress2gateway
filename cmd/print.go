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

type PrintRunner struct {
	// outputFormat contains currently set output format. Value assigned via --output/-o flag.
	// Defaults to YAML.
	outputFormat string

	// The path to the input yaml config file. Value assigned via --input_file flag
	inputFile string

	// The namespace used to query Gateway API objects. Value assigned via
	// --namespace/-n flag.
	// On absence, the current user active namespace is used.
	namespace string

	// allNamespaces indicates whether all namespaces should be used. Value assigned via
	// --all-namespaces/-A flag.
	allNamespaces bool

	// resourcePrinter determines how resource objects are printed out
	resourcePrinter printers.ResourcePrinter

	// Only resources that matches this filter will be processed.
	namespaceFilter string
}

func NewDefaultPrintRunner() *PrintRunner {
	pr := &PrintRunner{}
	pr.outputFormat = "yaml"
	return pr
}

func (pr *PrintRunner) PrintGatewaysAndHttpRoutes(cmd *cobra.Command, args []string) error {
	i2gw.Run(pr.resourcePrinter, pr.namespaceFilter, pr.inputFile)
	return nil
}

// InitializeResourcePrinter assign a specific type of printers.ResourcePrinter
// based on the outputFormat of the printRunner struct.
func (pr *PrintRunner) InitializeResourcePrinter() error {
	switch pr.outputFormat {
	case "yaml", "":
		pr.resourcePrinter = &printers.YAMLPrinter{}
		return nil
	case "json":
		pr.resourcePrinter = &printers.JSONPrinter{}
		return nil
	default:
		return fmt.Errorf("%s is not a supported output format", pr.outputFormat)
	}

}

func (pr *PrintRunner) InitializeNamespaceFilter() error {
	// When we should use all namespaces, empty string is used as the filter.
	if pr.allNamespaces {
		pr.namespaceFilter = ""
		return nil
	}

	if pr.namespace == "" {
		ns, err := getNamespaceInCurrentContext()
		if err != nil && pr.inputFile == "" {
			// When asked to read from the cluster, but getting the current namespace
			// failed for whatever reason - do not process the request.
			return err
		}
		// If err is nil we got the right filtered namespace.
		// If the input file is specified, and we failed to get the namespace, use all namespaces.
		pr.namespaceFilter = ns
		return nil
	}

	pr.namespaceFilter = pr.namespace
	return nil
}

func newPrintCommand() *cobra.Command {
	pr := NewDefaultPrintRunner()
	var printFlags genericclioptions.JSONYamlPrintFlags
	allowedFormats := printFlags.AllowedFormats()

	// printCmd represents the print command. It prints HTTPRoutes and Gateways
	// generated from Ingress resources.
	var cmd = &cobra.Command{
		Use:   "print",
		Short: "Prints HTTPRoutes and Gateways generated from Ingress resources",
		RunE:  pr.PrintGatewaysAndHttpRoutes,
	}

	cmd.RunE = pr.PrintGatewaysAndHttpRoutes

	cmd.Flags().StringVarP(&pr.outputFormat, "output", "o", "yaml",
		fmt.Sprintf(`Output format. One of: (%s)`, strings.Join(allowedFormats, ", ")))

	cmd.Flags().StringVar(&pr.inputFile, "input_file", "",
		`Path to the manifest file. When set, the tool will read ingresses from the file instead of reading from the cluster. Supported files are yaml and json`)

	cmd.Flags().StringVarP(&pr.namespace, "namespace", "n", "",
		`If present, the namespace scope for this CLI request`)

	cmd.Flags().BoolVarP(&pr.allNamespaces, "all-namespaces", "A", false,
		`If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even
if specified with --namespace.`)

	cmd.MarkFlagsMutuallyExclusive("namespace", "all-namespaces")
	return cmd
}

// getNamespaceFilter returns a namespace filter, taking into consideration whether a specific
// namespace is requested, or all of them are.
func getNamespaceFilter(pr *PrintRunner) (string, error) {

	// When we should use all namespaces, return an empty string.
	// This is the first condition since it should override the requestedNamespace,
	// if specified.
	if pr.allNamespaces {
		return "", nil
	}

	if pr.namespace == "" {
		return getNamespaceInCurrentContext()
	}
	return pr.namespace, nil
}

// getNamespaceInCurrentContext returns the namespace in the current active context of the user.
func getNamespaceInCurrentContext() (string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	currentNamespace, _, err := kubeConfig.Namespace()

	return currentNamespace, err
}

func init() {
	printCmd := newPrintCommand()
	rootCmd.AddCommand(printCmd)
}
