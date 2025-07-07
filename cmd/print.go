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
	"slices"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/tools/clientcmd"

	// Call init function for the providers
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/apisix"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/cilium"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/gce"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/openapi3"

	// Call init for notifications
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

type PrintRunner struct {
	// outputFormat contains currently set output format. Value assigned via --output/-o flag.
	// Defaults to YAML.
	outputFormat string

	// The path to the input yaml config file. Value assigned via --input-file flag
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

	// providers indicates which providers are used to execute convert action.
	providers []string

	// Provider specific flags --<provider>-<flag>.
	providerSpecificFlags map[string]*string
}

// PrintGatewayAPIObjects performs necessary steps to digest and print
// converted Gateway API objects. The steps include reading from the source,
// construct ingresses and provider-specific resources, convert them, then print
// the Gateway API objects out.
func (pr *PrintRunner) PrintGatewayAPIObjects(cmd *cobra.Command, _ []string) error {
	err := pr.initializeResourcePrinter()
	if err != nil {
		return fmt.Errorf("failed to initialize resrouce printer: %w", err)
	}
	err = pr.initializeNamespaceFilter()
	if err != nil {
		return fmt.Errorf("failed to initialize namespace filter: %w", err)
	}

	gatewayResources, notificationTablesMap, err := i2gw.ToGatewayAPIResources(cmd.Context(), pr.namespaceFilter, pr.inputFile, pr.providers, pr.getProviderSpecificFlags())
	if err != nil {
		return err
	}

	for _, table := range notificationTablesMap {
		fmt.Println(table)
	}

	pr.outputResult(gatewayResources)

	return nil
}

func (pr *PrintRunner) outputResult(gatewayResources []i2gw.GatewayResources) {
	resourceCount := 0

	for _, r := range gatewayResources {
		resourceCount += len(r.GatewayClasses)
		for _, gatewayClass := range r.GatewayClasses {
			gatewayClass := gatewayClass
			err := pr.resourcePrinter.PrintObj(&gatewayClass, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s GatewayClass: %v\n", gatewayClass.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.Gateways)
		for _, gateway := range r.Gateways {
			gateway := gateway
			if gateway.Annotations == nil {
				gateway.Annotations = make(map[string]string)
			}
			gateway.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&gateway, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s Gateway: %v\n", gateway.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.HTTPRoutes)
		for _, httpRoute := range r.HTTPRoutes {
			httpRoute := httpRoute
			if httpRoute.Annotations == nil {
				httpRoute.Annotations = make(map[string]string)
			}
			httpRoute.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&httpRoute, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s HTTPRoute: %v\n", httpRoute.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.GRPCRoutes)
		for _, grpcRoute := range r.GRPCRoutes {
			grpcRoute := grpcRoute
			if grpcRoute.Annotations == nil {
				grpcRoute.Annotations = make(map[string]string)
			}
			grpcRoute.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&grpcRoute, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s GRPCRoute: %v\n", grpcRoute.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.TLSRoutes)
		for _, tlsRoute := range r.TLSRoutes {
			tlsRoute := tlsRoute
			if tlsRoute.Annotations == nil {
				tlsRoute.Annotations = make(map[string]string)
			}
			tlsRoute.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&tlsRoute, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s TLSRoute: %v\n", tlsRoute.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.TCPRoutes)
		for _, tcpRoute := range r.TCPRoutes {
			tcpRoute := tcpRoute
			if tcpRoute.Annotations == nil {
				tcpRoute.Annotations = make(map[string]string)
			}
			tcpRoute.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&tcpRoute, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s TCPRoute: %v\n", tcpRoute.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.UDPRoutes)
		for _, udpRoute := range r.UDPRoutes {
			udpRoute := udpRoute
			if udpRoute.Annotations == nil {
				udpRoute.Annotations = make(map[string]string)
			}
			udpRoute.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&udpRoute, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s UDPRoute: %v\n", udpRoute.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.BackendTLSPolicies)
		for _, backendTLSPolicy := range r.BackendTLSPolicies {
			backendTLSPolicy := backendTLSPolicy
			if backendTLSPolicy.Annotations == nil {
				backendTLSPolicy.Annotations = make(map[string]string)
			}
			backendTLSPolicy.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&backendTLSPolicy, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s BackendTLSPolicy: %v\n", backendTLSPolicy.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.ReferenceGrants)
		for _, referenceGrant := range r.ReferenceGrants {
			referenceGrant := referenceGrant
			if referenceGrant.Annotations == nil {
				referenceGrant.Annotations = make(map[string]string)
			}
			referenceGrant.Annotations[i2gw.GeneratorAnnotationKey] = fmt.Sprintf("ingress2gateway-%s", i2gw.Version)
			err := pr.resourcePrinter.PrintObj(&referenceGrant, os.Stdout)
			if err != nil {
				fmt.Printf("# Error printing %s ReferenceGrant: %v\n", referenceGrant.Name, err)
			}
		}
	}

	for _, r := range gatewayResources {
		resourceCount += len(r.GatewayExtensions)
		for _, gatewayExtension := range r.GatewayExtensions {
			gatewayExtension := gatewayExtension
			fmt.Println("---")
			if err := PrintUnstructuredAsYaml(&gatewayExtension); err != nil {
				fmt.Printf("# Error printing %s gatewayExtension: %v\n", gatewayExtension.GetName(), err)
			}
		}
	}

	if resourceCount == 0 {
		msg := "No resources found"
		if pr.namespaceFilter != "" {
			msg = fmt.Sprintf("%s in %s namespace", msg, pr.namespaceFilter)
		}
		fmt.Println(msg)
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
// 3. If no namespace is specified and reading from the cluster, it attempts to get the namespace from the cluster; if unsuccessful, initialization fails.
// 4. If no namespace is specified and reading from a file, it attempts to get the namespace from the cluster; if unsuccessful, it reads all resources.
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

	// printCmd represents the print command. It prints Gateway API resources
	// generated from Ingress resources.
	var cmd = &cobra.Command{
		Use:   "print",
		Short: "Prints Gateway API objects generated from ingress and provider-specific resources.",
		RunE:  pr.PrintGatewayAPIObjects,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			openAPIExist := slices.Contains(pr.providers, "openapi3")
			if openAPIExist && len(pr.providers) != 1 {
				return fmt.Errorf("openapi3 must be the only provider when specified")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&pr.outputFormat, "output", "o", "yaml",
		fmt.Sprintf(`Output format. One of: (%s).`, strings.Join(allowedFormats, ", ")))

	cmd.Flags().StringVar(&pr.inputFile, "input-file", "",
		`Path to the manifest file. When set, the tool will read ingresses from the file instead of reading from the cluster. Supported files are yaml and json.`)

	cmd.Flags().StringVarP(&pr.namespace, "namespace", "n", "",
		`If present, the namespace scope for this CLI request.`)

	cmd.Flags().BoolVarP(&pr.allNamespaces, "all-namespaces", "A", false,
		`If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even
if specified with --namespace.`)

	cmd.Flags().StringSliceVar(&pr.providers, "providers", []string{},
		fmt.Sprintf("If present, the tool will try to convert only resources related to the specified providers, supported values are %v.", i2gw.GetSupportedProviders()))

	pr.providerSpecificFlags = make(map[string]*string)
	for provider, flags := range i2gw.GetProviderSpecificFlagDefinitions() {
		for _, flag := range flags {
			flagName := fmt.Sprintf("%s-%s", provider, flag.Name)
			pr.providerSpecificFlags[flagName] = cmd.Flags().String(flagName, flag.DefaultValue, fmt.Sprintf("Provider-specific: %s. %s", provider, flag.Description))
		}
	}

	_ = cmd.MarkFlagRequired("providers")
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

// getProviderSpecificFlags returns the provider specific flags input by the user.
// The flags are returned in a map where the key is the provider name and the value is a map of flag name to flag value.
func (pr *PrintRunner) getProviderSpecificFlags() map[string]map[string]string {
	providerSpecificFlags := make(map[string]map[string]string)
	for flagName, value := range pr.providerSpecificFlags {
		provider, found := lo.Find(pr.providers, func(p string) bool { return strings.HasPrefix(flagName, fmt.Sprintf("%s-", p)) })
		if !found {
			continue
		}
		flagNameWithoutProvider := strings.TrimPrefix(flagName, fmt.Sprintf("%s-", provider))
		if providerSpecificFlags[provider] == nil {
			providerSpecificFlags[provider] = make(map[string]string)
		}
		providerSpecificFlags[provider][flagNameWithoutProvider] = *value
	}
	return providerSpecificFlags
}

func PrintUnstructuredAsYaml(obj *unstructured.Unstructured) error {
	// Create a YAML serializer
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true, // Optional: for better readability
			Strict: true,
		})

	// Encode the unstructured object to YAML
	err := serializer.Encode(obj, os.Stdout)
	if err != nil {
		return err
	}

	return nil
}
