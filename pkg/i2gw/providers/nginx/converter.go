/*
Copyright 2025 The Kubernetes Authors.

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

package nginx

import (
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx/annotations"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx/crds"
)

type resourcesToIRConverter struct {
	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

func newResourcesToIRConverter() *resourcesToIRConverter {
	return &resourcesToIRConverter{
		featureParsers: []i2gw.FeatureParser{
			annotations.ListenPortsFeature,
			annotations.RewriteTargetFeature,
			annotations.HeaderManipulationFeature,
			annotations.PathRegexFeature,
			annotations.SSLRedirectFeature,
			annotations.HSTSFeature,
			annotations.WebSocketServicesFeature,
			annotations.SSLServicesFeature,
			annotations.GRPCServicesFeature,
		},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{},
	}
}

func (c *resourcesToIRConverter) convert(storage *storage) (intermediate.IR, field.ErrorList) {
	ingressList := []networkingv1.Ingress{}
	for _, ingress := range storage.Ingresses {
		if ingress != nil {
			ingressList = append(ingressList, *ingress)
		}
	}

	ir, errorList := common.ToIR(ingressList, storage.ServicePorts, c.implementationSpecificOptions)
	if len(errorList) > 0 {
		return intermediate.IR{}, errorList
	}

	// Convert all NGINX CRDs (VirtualServer, VirtualServerRoute, TransportServer) to IR
	crdIR, crdNotifications, errs := crds.CRDsToGatewayIR(
		storage.VirtualServers,
		storage.VirtualServerRoutes,
		storage.TransportServers,
		storage.GlobalConfiguration,
	)
	if len(errs) > 0 {
		errorList = append(errorList, errs...)
	}

	// Log CRD conversion notifications
	for _, notification := range crdNotifications {
		notify(notification.Type, notification.Message)
	}

	if len(errorList) > 0 {
		return intermediate.IR{}, errorList
	}

	// Merge CRD IR with Ingress IR
	ir, errs = intermediate.MergeIRs(ir, crdIR)
	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		errs := parseFeatureFunc(ingressList, storage.ServicePorts, &ir)
		errorList = append(errorList, errs...)
	}

	return ir, errorList
}
