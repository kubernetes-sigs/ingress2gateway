/*
Copyright 2022 The Kubernetes Authors.

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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// converter implements the ConvertHTTPRoutes function of i2gw.ResourceConverter interface.
type converter struct {
	conf *i2gw.ProviderConf

	featureParsers []i2gw.FeatureParser
}

// newConverter returns an ingress-nginx converter instance.
func newConverter(conf *i2gw.ProviderConf) *converter {
	return &converter{
		conf: conf,
		featureParsers: []i2gw.FeatureParser{
			canaryFeature,
		},
	}
}

// IngressToGateway converts the received i2gw.IngressResources to i2gw.GatewayResources
// including the ingress-nginx specific features.
func (c *converter) IngressToGateway(resources i2gw.IngressResources) (i2gw.GatewayResources, field.ErrorList) {

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	gatewayResources, errs := common.IngressToGateway(resources.Ingresses)
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	// Apply all patch functions on the gateway resources, one by one.
	for _, parseFeature := range c.featureParsers {
		errs = append(errs, parseFeature(resources, &gatewayResources)...)
	}

	return gatewayResources, errs
}
