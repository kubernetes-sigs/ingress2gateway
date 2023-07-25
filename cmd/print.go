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
	"os"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/spf13/cobra"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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

func (pr *PrintRunner) PrintGatewaysAndHttpRoutes(cmd *cobra.Command, args []string) error {
	ingressList := &networkingv1.IngressList{}
	if pr.inputFile != "" {
		err := i2gw.ConstructIngressesFromFile(ingressList, pr.inputFile, pr.namespace)
		if err != nil {
			fmt.Printf("failed to open input file: %v\n", err)
			os.Exit(1)
		}
	} else {
		cl := pr.GetNamespacedClient()
		err := i2gw.ConstructIngressesFromKubeCluster(cl, ingressList)
		if err != nil {
			fmt.Printf("failed to get ingress resources from kubenetes cluster: %v\n", err)
			os.Exit(1)
		}
	}

	if len(ingressList.Items) == 0 {
		msg := "No resources found"
		if pr.namespace != "" {
			fmt.Printf("%s in %s namespace\n", msg, pr.namespace)
			os.Exit(1)
		} else {
			fmt.Print(msg)
			os.Exit(1)
		}
	}

	httpRoutes, gateways, errors := i2gw.Ingresses2GatewaysAndHTTPRoutes(ingressList.Items)
	if len(errors) > 0 {
		fmt.Printf("# Encountered %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("# %s\n", err)
		}
		os.Exit(1)
	}

	i2gw.OutputResult(pr.resourcePrinter, httpRoutes, gateways)

	return nil
}

func (pr *PrintRunner) GetNamespacedClient() client.Client {
	conf, err := config.GetConfig()
	if err != nil {
		fmt.Println("failed to get client config")
		os.Exit(1)
	}

	cl, err := client.New(conf, client.Options{})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}
	if pr.namespace == "" {
		fmt.Println("failed to get client config because no namespace was specified")
		os.Exit(1)
	}
	return client.NewNamespacedClient(cl, pr.namespace)
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

// InitializeNamespaceFilter generate the corret namespace filter, taking into consideration whether a specific
// namespace is requested, or all of them are.
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
	pr := PrintRunner{}
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
	pr.InitializeResourcePrinter()
	pr.InitializeNamespaceFilter()
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
