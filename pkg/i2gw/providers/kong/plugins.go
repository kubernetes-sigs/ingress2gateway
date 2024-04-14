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
	"errors"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// pluginsFeature parses the Kong Ingress Controller plugins annotation and converts it
// into HTTPRoutes rule's ExtensionRef filters.
// It's possible to define a list of plugins to attach to the same HTTPRoute by setting
// a comma-separated list.
//
// Example: konghq.com/plugins: "plugin1,plugin2"
func pluginsFeature(ingressResources i2gw.InputResources, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingressResources.Ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			key := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRoute, ok := gatewayResources.HTTPRoutes[key]
			if !ok {
				return field.ErrorList{field.InternalError(nil, errors.New("HTTPRoute does not exist - this should never happen"))}
			}
			filters := parsePluginsAnnotation(rule.Ingress.Annotations)
			patchHTTPRoutePlugins(&httpRoute, filters)
		}
	}
	return nil
}

func parsePluginsAnnotation(annotations map[string]string) []gatewayv1.HTTPRouteFilter {
	filters := make([]gatewayv1.HTTPRouteFilter, 0)
	mkey := kongAnnotation(pluginsKey)
	for key, val := range annotations {
		if key == mkey {
			filtersValues := strings.Split(val, ",")
			for _, v := range filtersValues {
				if v == "" {
					continue
				}
				filters = append(filters, gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterExtensionRef,
					ExtensionRef: &gatewayv1.LocalObjectReference{
						Group: gatewayv1.Group(kongResourcesGroup),
						Kind:  gatewayv1.Kind(kongPluginKind),
						Name:  gatewayv1.ObjectName(v),
					},
				})
			}
		}
	}
	return filters
}

func patchHTTPRoutePlugins(httpRoute *gatewayv1.HTTPRoute, extensionRefs []gatewayv1.HTTPRouteFilter) {
	for i := range httpRoute.Spec.Rules {
		if httpRoute.Spec.Rules[i].Filters == nil {
			httpRoute.Spec.Rules[i].Filters = make([]gatewayv1.HTTPRouteFilter, 0)
		}
		httpRoute.Spec.Rules[i].Filters = append(httpRoute.Spec.Rules[i].Filters, extensionRefs...)
	}
}
