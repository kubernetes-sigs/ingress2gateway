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
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong/crds"
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

// newConverter returns an kong converter instance.
func newConverter() *converter {
	return &converter{
		featureParsers: []i2gw.FeatureParser{
			headerMatchingFeature,
			methodMatchingFeature,
			pluginsFeature,
		},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			ToImplementationSpecificHTTPPathTypeMatch: implementationSpecificHTTPPathTypeMatch,
		},
	}
}

func (c *converter) convert(storage *storage) (i2gw.GatewayResources, field.ErrorList) {
	ingressList := []networkingv1.Ingress{}
	for _, ingress := range storage.Ingresses {
		ingressList = append(ingressList, *ingress)
	}

	globalErrs := field.ErrorList{}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	gatewayResources, errs := common.ToGateway(ingressList, c.implementationSpecificOptions)
	if len(errs) > 0 {
		globalErrs = append(globalErrs, errs...)
	}

	tcpGatewayResources, errs := crds.TCPIngressToGatewayAPI(storage.TCPIngresses)
	if len(errs) > 0 {
		globalErrs = append(globalErrs, errs...)
	}

	gatewayResources, errs = i2gw.MergeGatewayResources(gatewayResources, tcpGatewayResources)
	if len(errs) > 0 {
		globalErrs = append(globalErrs, errs...)
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		errs = parseFeatureFunc(i2gw.InputResources{Ingresses: ingressList}, &gatewayResources)
		// Append the parsing errors to the error list.
		globalErrs = append(globalErrs, errs...)
	}

	return gatewayResources, globalErrs
}
