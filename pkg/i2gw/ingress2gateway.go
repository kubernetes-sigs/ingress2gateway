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

package i2gw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func Run(printer printers.ResourcePrinter, namespace string, inputFile string) {
	ingressList := &networkingv1.IngressList{}
	if inputFile != "" {
		err := constructIngressesFromFile(ingressList, inputFile, namespace)
		if err != nil {
			fmt.Printf("failed to open input file: %v\n", err)
			os.Exit(1)
		}
	} else {
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
		cl = client.NewNamespacedClient(cl, namespace)

		err = cl.List(context.Background(), ingressList)
		if err != nil {
			fmt.Printf("failed to list ingresses: %v\n", err)
			os.Exit(1)
		}
	}

	if len(ingressList.Items) == 0 {
		msg := "No resources found"
		if namespace != "" {
			fmt.Printf("%s in %s namespace\n", msg, namespace)
		} else {
			fmt.Println(msg)
		}
		return
	}

	httpRoutes, gateways, errors := ingresses2GatewaysAndHTTPRoutes(ingressList.Items)
	if len(errors) > 0 {
		fmt.Printf("# Encountered %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("# %s\n", err)
		}
		return
	}

	outputResult(printer, httpRoutes, gateways)
}

func ingresses2GatewaysAndHTTPRoutes(ingresses []networkingv1.Ingress) ([]gatewayv1beta1.HTTPRoute, []gatewayv1beta1.Gateway, field.ErrorList) {
	aggregator := ingressAggregator{ruleGroups: map[ruleGroupKey]*ingressRuleGroup{}}

	var errs field.ErrorList
	for _, ingress := range ingresses {
		errs = append(errs, aggregator.addIngress(ingress)...)
	}
	if len(errs) > 0 {
		return nil, nil, errs
	}

	return aggregator.toHTTPRoutesAndGateways()
}

func outputResult(printer printers.ResourcePrinter, httpRoutes []gatewayv1beta1.HTTPRoute, gateways []gatewayv1beta1.Gateway) {
	for i := range gateways {
		err := printer.PrintObj(&gateways[i], os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", gateways[i].Name, err)
		}
	}

	for i := range httpRoutes {
		err := printer.PrintObj(&httpRoutes[i], os.Stdout)
		if err != nil {
			fmt.Printf("# Error printing %s HTTPRoute: %v\n", httpRoutes[i].Name, err)
		}
	}
}

// extractObjectsFromReader extracts all objects from a reader,
// which is created from YAML or JSON input files.
// It retrieves all objects, including nested ones if they are contained within a list.
func extractObjectsFromReader(reader io.Reader) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(reader, 4096)
	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := d.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		if u == nil {
			continue
		}
		objs = append(objs, u)
	}

	finalObjs := []*unstructured.Unstructured{}
	for _, obj := range objs {
		tmpObjs := []*unstructured.Unstructured{}
		if obj.IsList() {
			err := obj.EachListItem(func(object runtime.Object) error {
				unstructuredObj, ok := object.(*unstructured.Unstructured)
				if ok {
					tmpObjs = append(tmpObjs, unstructuredObj)
					return nil
				}
				return fmt.Errorf("resource list item has unexpected type")
			})
			if err != nil {
				return nil, err
			}
		} else {
			tmpObjs = append(tmpObjs, obj)
		}
		finalObjs = append(finalObjs, tmpObjs...)
	}

	return finalObjs, nil
}

// constructIngressesFromFile reads the inputFile in either json/yaml formats,
// then deserialize the file into Ingresses resources.
// All ingresses will be pushed into the supplied IngressList for return.
func constructIngressesFromFile(l *networkingv1.IngressList, inputFile string, namespace string) error {
	stream, err := os.ReadFile(inputFile)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(stream)
	objs, err := extractObjectsFromReader(reader)
	if err != nil {
		return err
	}

	for _, f := range objs {
		if namespace != "" && f.GetNamespace() != namespace {
			continue
		}
		if !f.GroupVersionKind().Empty() && f.GroupVersionKind().Kind == "Ingress" {
			var i networkingv1.Ingress
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &i)
			if err != nil {
				return err
			}
			l.Items = append(l.Items, i)
		}

	}
	return nil
}
