/*
Copyright The Kubernetes Authors.

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

package traefik

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// resourcesToIRConverter implements the ToIR function of i2gw.ResourcesToIRConverter interface.
type resourcesToIRConverter struct {
	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
	notify                        notifications.NotifyFunc
}

// newResourcesToIRConverter returns a traefik resourcesToIRConverter instance.
func newResourcesToIRConverter(notify notifications.NotifyFunc) *resourcesToIRConverter {
	return &resourcesToIRConverter{
		// Order matters: routerTLSFeature → routerEntrypointsFeature → forceHTTPSFeature.
		featureParsers: []i2gw.FeatureParser{
			routerTLSFeature,
			routerEntrypointsFeature,
			forceHTTPSFeature,
			unsupportedAnnotationsFeature,
		},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			ToImplementationSpecificHTTPPathTypeMatch: implementationSpecificPathMatch,
		},
		notify: notify,
	}
}

func (c *resourcesToIRConverter) convertToIR(storage *storage) (providerir.ProviderIR, field.ErrorList) {
	ingressList := []networkingv1.Ingress{}
	for _, ing := range storage.Ingresses {
		ingressList = append(ingressList, *ing)
	}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	ir, errs := common.ToIR(ingressList, []networkingv1.Ingress{}, storage.ServicePorts, c.implementationSpecificOptions)
	if len(errs) > 0 {
		return providerir.ProviderIR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(c.notify, ingressList, storage.ServicePorts, &ir)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return ir, errs
}

// implementationSpecificPathMatch converts ImplementationSpecific path type to
// PathPrefix. Traefik treats ImplementationSpecific as prefix matching by default,
// so this is the correct and safe mapping.
func implementationSpecificPathMatch(path *gatewayv1.HTTPPathMatch) {
	t := gatewayv1.PathMatchPathPrefix
	path.Type = &t
}
