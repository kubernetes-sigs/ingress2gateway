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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// resourcesToIRConverter implements the ToIR function of i2gw.ResourcesToIRConverter interface.
type resourcesToIRConverter struct {
	featureParsers []i2gw.FeatureParser
}

// newResourcesToIRConverter returns an ingress-nginx resourcesToIRConverter instance.
func newResourcesToIRConverter() *resourcesToIRConverter {
	return &resourcesToIRConverter{
		featureParsers: []i2gw.FeatureParser{
			// canaryFeature,
		},
	}
}

func IngressNginxHTTPRuleConverter(paths []i2gw.IngressPath, fieldPath *field.Path, servicePorts map[types.NamespacedName]map[string]int32, namespace string) ([]gatewayv1.HTTPRouteRule, field.ErrorList) {
	if len(paths) == 0 {
		return nil, nil
	}
	var errors field.ErrorList
	numCanaryPaths := 0
	parsedPaths := make([]ingressPathWithCanary, 0, len(paths))
	for _, path := range paths {
		var canary *canaryAnnotations
		if path.Ingress != nil {
			c, errs := parseCanaryAnnotations(path.Ingress)
			if len(errs) > 0 {
				return nil, errs
			}
			if c.enable {
				canary = &c
				numCanaryPaths++
			}
		}
		parsedPaths = append(parsedPaths, ingressPathWithCanary{
			path:   &path,
			canary: canary,
		})
	}
	if numCanaryPaths > 1 || len(parsedPaths)-numCanaryPaths > 1 {
		errors = append(errors, field.Invalid(fieldPath, parsedPaths, "ingress nginx canary mode supports only one canary path or one stable path per HTTPRoute"))
		// notifications.Warnf("ingress-nginx canary mode supports only one canary path or one stable path per HTTPRoute, found %d canary paths and %d stable paths in ingress %s/%s", numCanaryPaths, len(parsedPaths)-numCanaryPaths, paths[0].NamespacedName().Namespace, paths[0].NamespacedName().Name)
		return nil, errors
	}

	// No canary, this is easy
	if numCanaryPaths == 0 {
		match, err := common.ToHTTPRouteMatch(paths[0].Path, fieldPath, nil)
		if err != nil {
			errors = append(errors, err)
			return nil, errors
		}
		hrRule := gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{*match},
		}
		backendRefs, errs := common.ConfigureBackendRef(servicePorts, paths, namespace)
		errors = append(errors, errs...)
		hrRule.BackendRefs = backendRefs
		return []gatewayv1.HTTPRouteRule{hrRule}, errors
	}

	var stablePath, canaryPath *ingressPathWithCanary
	var stableIndex, canaryIndex int
	for i := range parsedPaths {
		if parsedPaths[i].canary != nil && parsedPaths[i].canary.enable {
			canaryPath = &parsedPaths[i]
			canaryIndex = i
		} else {
			stablePath = &parsedPaths[i]
			stableIndex = i
		}
	}

	// check if we are doing header based canary
	if canaryPath.canary.headerKey != "" {
		// check by value
		var canaryMatches []gatewayv1.HTTPRouteMatch
		if !canaryPath.canary.headerRegexMatch {
			canaryMatches = append(canaryMatches, gatewayv1.HTTPRouteMatch{
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Name:  gatewayv1.HTTPHeaderName(canaryPath.canary.headerKey),
						Value: canaryPath.canary.headerValue,
						Type:  common.PtrTo(gatewayv1.HeaderMatchExact),
					},
				},
			})
		} else {
			canaryMatches = append(canaryMatches, gatewayv1.HTTPRouteMatch{
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Name:  gatewayv1.HTTPHeaderName(canaryPath.canary.headerKey),
						Value: canaryPath.canary.headerValue,
						Type:  common.PtrTo(gatewayv1.HeaderMatchRegularExpression),
					},
				},
			})
		}

		baseMatch, err := common.ToHTTPRouteMatch(paths[0].Path, fieldPath, nil)
		if err != nil {
			errors = append(errors, err)
			return nil, errors
		}
		stableHRRule := gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{*baseMatch},
		}
		canaryMatches = append(canaryMatches, *baseMatch)
		canaryHRRule := gatewayv1.HTTPRouteRule{
			Matches: canaryMatches,
		}
		stableBackendRefs, errs := common.ConfigureBackendRef(servicePorts, []i2gw.IngressPath{*stablePath.path}, namespace)
		errors = append(errors, errs...)
		canaryBackendRefs, errs := common.ConfigureBackendRef(servicePorts, []i2gw.IngressPath{*canaryPath.path}, namespace)
		errors = append(errors, errs...)
		stableHRRule.BackendRefs = stableBackendRefs
		canaryHRRule.BackendRefs = canaryBackendRefs
		return []gatewayv1.HTTPRouteRule{canaryHRRule, stableHRRule}, errors // canary first
	}

	// we just do weight based canary
	stableWeight := canaryPath.canary.weightTotal - canaryPath.canary.weight
	canaryWeight := canaryPath.canary.weight
	match, err := common.ToHTTPRouteMatch(paths[0].Path, fieldPath, nil)
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}
	hrRule := gatewayv1.HTTPRouteRule{
		Matches: []gatewayv1.HTTPRouteMatch{*match},
	}
	canaryBackendRef, err := common.ToBackendRef(namespace, canaryPath.path.Path.Backend, servicePorts, field.NewPath("paths", "backends").Index(canaryIndex))
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}
	canaryBackendRef.Weight = common.PtrTo(int32(canaryWeight))
	stableBackendRef, err := common.ToBackendRef(namespace, stablePath.path.Path.Backend, servicePorts, field.NewPath("paths", "backends").Index(stableIndex))
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}
	stableBackendRef.Weight = common.PtrTo(int32(stableWeight))
	hrRule.BackendRefs = []gatewayv1.HTTPBackendRef{
		{BackendRef: *stableBackendRef},
		{BackendRef: *canaryBackendRef},
	}
	return []gatewayv1.HTTPRouteRule{hrRule}, errors

}

func (c *resourcesToIRConverter) convert(storage *storage) (intermediate.IR, field.ErrorList) {

	// TODO(liorliberman) temporary until we decide to change ToIR and featureParsers to get a map of [types.NamespacedName]*networkingv1.Ingress instead of a list
	ingressList := storage.Ingresses.List()

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	ir, errs := common.ToIR(ingressList, storage.ServicePorts, i2gw.ProviderImplementationSpecificOptions{
		ToImplementationSpecificRules: IngressNginxHTTPRuleConverter,

	})
	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(ingressList, storage.ServicePorts, &ir)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return ir, errs
}
