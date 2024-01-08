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

package apisix

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

// newConverter returns an apisix converter instance.
func newConverter() *converter {
	return &converter{
		featureParsers: []i2gw.FeatureParser{
			httpToHTTPSFeature,
		},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			// The list of the implementationSpecific ingress fields options comes here.
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

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(i2gw.InputResources{Ingresses: ingressList}, &gatewayResources)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return gatewayResources, errs
}
