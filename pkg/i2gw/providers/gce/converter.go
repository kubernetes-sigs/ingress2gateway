/*
Copyright 2024 The Kubernetes Authors.

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

package gce

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	conf *i2gw.ProviderConf

	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

// newConverter returns an ingress-gce converter instance.
func newConverter(conf *i2gw.ProviderConf) converter {
	return converter{
		conf:           conf,
		featureParsers: []i2gw.FeatureParser{},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			ToImplementationSpecificHTTPPathTypeMatch: implementationSpecificHTTPPathTypeMatch,
		},
	}
}

func (c *converter) convert(storage *storage) (i2gw.GatewayResources, field.ErrorList) {
	ingressList := []networkingv1.Ingress{}
	for _, ing := range storage.Ingresses {
		ingressList = append(ingressList, *ing)
	}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	gatewayResources, errs := common.ToGateway(ingressList, c.implementationSpecificOptions)
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	errs = setGCEGatewayClasses(ingressList, &gatewayResources)
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(ingressList, &gatewayResources)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return gatewayResources, errs
}
