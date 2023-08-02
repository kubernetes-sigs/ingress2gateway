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

package datasource

import (
	"fmt"
	"os"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type DataSource struct {
	NamespaceFilter string
	InputFile       string
}

func (ds *DataSource) GetIngessList() (*networkingv1.IngressList, error) {
	ingressList := &networkingv1.IngressList{}
	if ds.InputFile != "" {
		err := i2gw.ConstructIngressesFromFile(ingressList, ds.InputFile, ds.NamespaceFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to open input file: %v\n", err)
		}
	} else {
		cl := ds.getNamespacedClient()
		err := i2gw.ConstructIngressesFromCluster(cl, ingressList)
		if err != nil {
			return nil, fmt.Errorf("failed to get ingress resources from kubenetes cluster: %v\n", err)
		}
	}

	if len(ingressList.Items) == 0 {
		msg := "No resources found"
		if ds.NamespaceFilter != "" {
			return nil, fmt.Errorf("%s in %s namespace\n", msg, ds.NamespaceFilter)
		} else {
			return nil, fmt.Errorf(msg)
		}
	}
	return ingressList, nil
}

func (ds *DataSource) getNamespacedClient() client.Client {
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
	if ds.NamespaceFilter == "" {
		fmt.Println("failed to get client config because no namespace was specified")
		os.Exit(1)
	}
	return client.NewNamespacedClient(cl, ds.NamespaceFilter)
}
