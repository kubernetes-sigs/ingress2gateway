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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong/crds"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	configurationv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	conf *i2gw.ProviderConf

	featureParsers []i2gw.FeatureParser
}

// newConverter returns an kong converter instance.
func newConverter(conf *i2gw.ProviderConf) *converter {
	return &converter{
		conf: conf,
		featureParsers: []i2gw.FeatureParser{
			headerMatchingFeature,
			methodMatchingFeature,
			pluginsFeature,
		},
	}
}

// ToGatewayAPI converts the received i2gw.InputResources to i2gw.GatewayResources
// including the kong specific features.
func (c *converter) ToGatewayAPI(resources i2gw.InputResources) (i2gw.GatewayResources, field.ErrorList) {

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	gatewayResources, errs := common.ToGateway(resources.Ingresses)
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	if tcpIngresses, ok := resources.CustomResources[schema.GroupVersionKind{
		Group:   string(kongResourcesGroup),
		Kind:    string(kongTCPIngressKind),
		Version: "v1beta1",
	}].([]configurationv1beta1.TCPIngress); ok {
		tcpGatewayResources, errs := crds.TcpIngressToGatewayAPI(tcpIngresses)
		if errs != nil {
			return i2gw.GatewayResources{}, errs
		}
		gatewayResources = i2gw.MergeGatewayResources(gatewayResources, tcpGatewayResources)
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(resources, &gatewayResources)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return gatewayResources, errs
}
