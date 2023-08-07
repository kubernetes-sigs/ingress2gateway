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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/datasource"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/tools/clientcmd"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
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

// PrintGatewaysAndHttpRoutes performs necessary steps to digest and print
// converted Gateways and HTTP Routes. The steps includes reading from the source,
// construct ingresses, convert them, then print them out.
func (pr *PrintRunner) PrintGatewaysAndHttpRoutes(cmd *cobra.Command, args []string) error {
	err := pr.initializeResourcePrinter()
	if err != nil {
		return fmt.Errorf("Failed to initialize resrouce printer: %v", err)
	}
	err = pr.initializeNamespaceFilter()
	if err != nil {
		return fmt.Errorf("Failed to initialize namespace filter: %v", err)
	}

	ds := datasource.DataSource{
		NamespaceFilter: pr.namespaceFilter,
		InputFile:       pr.inputFile,
	}
	ingressList, err := ds.GetIngessList()
	if err != nil {
		return fmt.Errorf("Failed to get ingresses from source: %v", err)
	}

	httpRoutes, gateways, errList := i2gw.Ingresses2GatewaysAndHTTPRoutes(ingressList.Items)
	if len(errList) > 0 {
		errMsg := errors.New(fmt.Sprintf("# Encountered %d errors\n", len(errList)))
		for _, err := range errList {
			errMsg = fmt.Errorf("%w # %s\n", errMsg, err)
		}
		return errMsg
	}

	pr.outputResult(httpRoutes, gateways)

	return nil
}

func (pr *PrintRunner) outputResult(httpRoutes []gatewayv1beta1.HTTPRoute, gateways []gatewayv1beta1.Gateway) {
	for i := range gateways {
		err := pr.resourcePrinter.PrintObj(&gateways[i], os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", gateways[i].Name, err)
		}
	}

	for i := range httpRoutes {
		err := pr.resourcePrinter.PrintObj(&httpRoutes[i], os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", httpRoutes[i].Name, err)
		}
	}
}

// initializeResourcePrinter assign a specific type of printers.ResourcePrinter
// based on the outputFormat of the printRunner struct.
func (pr *PrintRunner) initializeResourcePrinter() error {
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

// initializeNamespaceFilter initializes the correct namespace filter for resource processing with these scenarios:
// 1. If the --all-namespaces flag is used, it processes all resources, regardless of whether they are from the cluster or file.
// 2. If namespace is specified, it filters resources based on that namespace.
// 3. If no namespace is specified and reading from the cluster, initialization fails; but if reading from a file, it reads all resources.
func (pr *PrintRunner) initializeNamespaceFilter() error {
	// When we should use all namespaces, empty string is used as the filter.
	if pr.allNamespaces {
		pr.namespaceFilter = ""
		return nil
	}

	// If namespace flag is not specified, try to use the default namespace from the cluster
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
	pr := &PrintRunner{}
	var printFlags genericclioptions.JSONYamlPrintFlags
	allowedFormats := printFlags.AllowedFormats()

	// printCmd represents the print command. It prints HTTPRoutes and Gateways
	// generated from Ingress resources.
	var cmd = &cobra.Command{
		Use:   "print",
		Short: "Prints HTTPRoutes and Gateways generated from Ingress resources",
		RunE:  pr.PrintGatewaysAndHttpRoutes,
	}

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

// getNamespaceInCurrentContext returns the namespace in the current active context of the user.
func getNamespaceInCurrentContext() (string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	currentNamespace, _, err := kubeConfig.Namespace()

	return currentNamespace, err
}

func init() {
	rootCmd.AddCommand(newPrintCommand())
}
