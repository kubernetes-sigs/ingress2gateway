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
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const implementationAnnotation = "ingress2gateway.kubernetes.io/implementation"

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
		},
	}
}

func (c *resourcesToIRConverter) convert(storage *storage) (intermediate.IR, field.ErrorList) {
	// TODO(liorliberman) temporary until we decide to change ToIR and featureParsers
	// to get a map of [types.NamespacedName]*networkingv1.Ingress instead of a list.
	ingressList := storage.Ingresses.List()

	// Derive implementation (if any) from Ingress annotations.
	opts := i2gw.ProviderImplementationSpecificOptions{
		GatewayClassNameOverride: detectImplementation(ingressList),
	}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	ir, errs := common.ToIR(ingressList, storage.ServicePorts, opts)
	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(ingressList, storage.ServicePorts, &ir)
		errs = append(errs, parseErrs...)
	}

	return ir, errs
}

func detectImplementation(ingresses []networkingv1.Ingress) string {
	impl := ""
	for _, ing := range ingresses {
		if ing.Annotations == nil {
			continue
		}
		if v, ok := ing.Annotations[implementationAnnotation]; ok && v != "" {
			if impl == "" {
				impl = v
			}
			// TODO [danehans]: log or collect a warning about conflicting
			// implementation annotations.
		}
	}
	return impl
}
