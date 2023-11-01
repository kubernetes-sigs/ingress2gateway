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

package ingressnginx

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

// The Name of the provider.
const Name = "ingress-nginx"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	conf *i2gw.ProviderConf

	*resourceReader
	*resourceFilter
	*converter
}

// NewProvider constructs and returns the ingress-nginx implementation of i2gw.Provider.
func NewProvider(conf i2gw.ProviderConf) i2gw.Provider {
	conf.FilteredObjects = filteredObjects
	return &Provider{
		conf:           &conf,
		resourceReader: newResourceReader(&conf),
		resourceFilter: newResourceFilter(&conf),
		converter:      newConverter(&conf),
	}
}
