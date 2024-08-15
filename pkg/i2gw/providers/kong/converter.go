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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong/crds"
)

// resourcesToIRConverter implements the ToIR function of i2gw.ResourcesToIRConverter interface.
type resourcesToIRConverter struct {
	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

// newResourcesToIRConverter returns an kong converter instance.
func newResourcesToIRConverter() *resourcesToIRConverter {
	return &resourcesToIRConverter{
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

func (c *resourcesToIRConverter) convert(storage *storage) (intermediate.IR, field.ErrorList) {
	ingressList := []networkingv1.Ingress{}
	for _, ingress := range storage.Ingresses {
		ingressList = append(ingressList, *ingress)
	}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	ir, errorList := common.ToIR(ingressList, c.implementationSpecificOptions, notify)
	if len(errorList) > 0 {
		return intermediate.IR{}, errorList
	}

	tcpGatewayIR, notificationsAggregator, errs := crds.TCPIngressToGatewayIR(storage.TCPIngresses)
	if len(errs) > 0 {
		errorList = append(errorList, errs...)
	}

	dispatchNotification(notificationsAggregator)

	if len(errorList) > 0 {
		return intermediate.IR{}, errorList
	}

	ir, errs = intermediate.MergeIRs(ir, tcpGatewayIR)

	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		errs = parseFeatureFunc(ingressList, &ir)
		// Append the parsing errors to the error list.
		errorList = append(errorList, errs...)
	}

	return ir, errorList
}
