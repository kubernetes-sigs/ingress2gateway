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
package i2gw

import (
	"context"
	"fmt"
	"os"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type RootRunner struct {
	ResourcePrinter printers.ResourcePrinter
}

func (r *RootRunner) Run() {
	cl, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	ingressList := &networkingv1.IngressList{}

	err = cl.List(context.Background(), ingressList)
	if err != nil {
		fmt.Printf("failed to list ingresses: %v\n", err)
		os.Exit(1)
	}

	httpRoutes, gateways, errors := ingresses2GatewaysAndHttpRoutes(ingressList.Items)
	if len(errors) > 0 {
		fmt.Printf("# Encountered %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("# %s", err)
		}
	}

	outputResult(r.ResourcePrinter, httpRoutes, gateways)
}

func ingresses2GatewaysAndHttpRoutes(ingresses []networkingv1.Ingress) ([]gatewayv1beta1.HTTPRoute, []gatewayv1beta1.Gateway, []error) {
	aggregator := ingressAggregator{ruleGroups: map[ruleGroupKey]*ingressRuleGroup{}}

	for _, ingress := range ingresses {
		aggregator.addIngress(ingress)
	}

	return aggregator.toHTTPRoutesAndGateways()
}

func outputResult(printer printers.ResourcePrinter, httpRoutes []gatewayv1beta1.HTTPRoute, gateways []gatewayv1beta1.Gateway) {
	for _, gateway := range gateways {
		err := printer.PrintObj(&gateway, os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", gateway.Name, err)
		}
	}

	for _, httpRoute := range httpRoutes {
		err := printer.PrintObj(&httpRoute, os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", httpRoute.Name, err)
		}
	}
}
