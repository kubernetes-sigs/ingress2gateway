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
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
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
	gatewayResources, errs := common.ToGateway(resources.Ingresses, toHTTPRouteMatchOption)
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(resources, &gatewayResources)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return gatewayResources, errs
}

func toHTTPRouteMatchOption(routePath networkingv1.HTTPIngressPath, path *field.Path) (*gatewayv1beta1.HTTPRouteMatch, *field.Error) {
	pmPrefix := gatewayv1beta1.PathMatchPathPrefix
	pmExact := gatewayv1beta1.PathMatchExact
	pmRegex := gatewayv1beta1.PathMatchRegularExpression

	match := &gatewayv1beta1.HTTPRouteMatch{Path: &gatewayv1beta1.HTTPPathMatch{Value: &routePath.Path}}
	switch *routePath.PathType {
	case networkingv1.PathTypePrefix:
		match.Path.Type = &pmPrefix
	case networkingv1.PathTypeExact:
		match.Path.Type = &pmExact
	case networkingv1.PathTypeImplementationSpecific:
		if strings.HasPrefix(routePath.Path, "/~") {
			match.Path.Type = &pmRegex
			match.Path.Value = common.PtrTo(strings.TrimPrefix(*match.Path.Value, "/~"))
		} else {
			match.Path.Type = &pmPrefix
		}

	default:
		return nil, field.Invalid(path.Child("pathType"), routePath.PathType, fmt.Sprintf("unsupported path match type: %s", *routePath.PathType))
	}

	return match, nil
}
