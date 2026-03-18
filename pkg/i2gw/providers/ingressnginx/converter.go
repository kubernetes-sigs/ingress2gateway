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
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw"
	providerir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// resourcesToIRConverter implements the ToIR function of i2gw.ResourcesToIRConverter interface.
type resourcesToIRConverter struct {
	featureParsers []i2gw.FeatureParser
}

// newResourcesToIRConverter returns an ingress-nginx resourcesToIRConverter instance.
func newResourcesToIRConverter() *resourcesToIRConverter {
	return &resourcesToIRConverter{
		featureParsers: []i2gw.FeatureParser{
			canaryFeature,
			bufferPolicyFeature,
			corsPolicyFeature,
			rateLimitPolicyFeature,
			proxyBodySizeFeature,
			proxySendTimeoutFeature,
			proxyReadTimeoutFeature,
			proxyConnectTimeoutFeature,
			enableAccessLogFeature,
			extAuthFeature,
			basicAuthFeature,
			sessionAffinityFeature,
			loadBalancingFeature,
			backendTLSFeature,
			serviceUpstreamFeature,
			backendProtocolFeature, // Must come after serviceUpstreamFeature.
			sslRedirectFeature,
			sslPassthroughFeature,
			rewriteTargetFeature,
			useRegexFeature,
			headerModifierFeature,
		},
	}
}

func (c *resourcesToIRConverter) convert(storage *storage) (providerir.ProviderIR, field.ErrorList) {

	// TODO(liorliberman) temporary until we decide to change ToIR and featureParsers to get a map of [types.NamespacedName]*networkingv1.Ingress instead of a list
	ingressList := storage.Ingresses.List()

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	ir, errs := common.ToIR(ingressList, storage.ServicePorts, i2gw.ProviderImplementationSpecificOptions{})
	if len(errs) > 0 {
		return providerir.ProviderIR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(ingressList, storage.ServicePorts, &ir)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	// Cross-feature validation that depends on derived host-wide regex mode.
	errs = append(errs, validateRegexCookiePath(&ir)...)

	return ir, errs
}
