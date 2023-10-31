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

package kong

import (
	"fmt"

	kongscheme "github.com/kong/kubernetes-ingress-controller/v2/pkg/clientset/scheme"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The Name of the provider.
const Name = "kong"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
	i2gw.ProviderClientBuilderByName[Name] = NewClient
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	conf *i2gw.ProviderConf

	*resourceReader
	*converter
}

// NewProvider constructs and returns the kong implementation of i2gw.Provider.
func NewProvider(conf i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		conf:           &conf,
		resourceReader: newResourceReader(&conf),
		converter:      newConverter(&conf),
	}
}

func NewClient(restConfig *rest.Config, namespace string) (client.Client, error) {
	cl, err := client.New(restConfig, client.Options{
		Scheme: kongscheme.Scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	cl = client.NewNamespacedClient(cl, namespace)
	return cl, nil
}
