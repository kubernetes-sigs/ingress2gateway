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
	"fmt"
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func canaryFeature(ingresses []networkingv1.Ingress, servicePorts map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)

	for _, rg := range ruleGroups {
		ingressPathsByMatchKey, errs := getPathsByMatchGroups(rg)
		if len(errs) > 0 {
			return errs
		}

		// We're dividing ingresses based on rule groups.  If any path within a
		// rule group is associated with an ingress object containing canary annotations,
		// the entire rule group is affected.
		canaryEnabled := false
		for _, paths := range ingressPathsByMatchKey {
			for _, path := range paths {
				if path.extra.canary.enable {
					canaryEnabled = true
				}
			}
		}

		if canaryEnabled {
			for _, paths := range ingressPathsByMatchKey {
				path := paths[0]

				backendRefs, calculationErrs := calculateBackendRefWeight(rg.Namespace, servicePorts, paths)
				errs = append(errs, calculationErrs...)

				key := types.NamespacedName{Namespace: path.ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
				httpRouteContext, ok := ir.HTTPRoutes[key]
				if !ok {
					// If there wasn't an HTTPRoute for this Ingress, we can skip it as something is wrong.
					// All the available errors will be returned at the end.
					continue
				}

				patchHTTPRouteWithBackendRefs(&httpRouteContext.HTTPRoute, backendRefs)
			}
			if len(errs) > 0 {
				return errs
			}
		}
	}

	return nil
}

func getPathsByMatchGroups(rg common.IngressRuleGroup) (map[pathMatchKey][]ingressPath, field.ErrorList) {
	ingressPathsByMatchKey := map[pathMatchKey][]ingressPath{}

	for _, ir := range rg.Rules {

		ingress := ir.Ingress
		annotations, errs := parseCanaryAnnotations(ingress)
		if len(errs) > 0 {
			return nil, errs
		}

		extraFeatures := extra{canary: &annotations}

		for _, path := range ir.IngressRule.HTTP.Paths {
			ip := ingressPath{ingress: ingress, ruleType: "http", path: path, extra: &extraFeatures}
			pmKey := getPathMatchKey(ip)
			ingressPathsByMatchKey[pmKey] = append(ingressPathsByMatchKey[pmKey], ip)
		}
	}

	return ingressPathsByMatchKey, nil
}

func patchHTTPRouteWithBackendRefs(httpRoute *gatewayv1.HTTPRoute, backendRefs []gatewayv1.HTTPBackendRef) {
	var ruleExists bool
	for _, backendRef := range backendRefs {

		ruleExists = false

		for _, rule := range httpRoute.Spec.Rules {
			for i := range rule.BackendRefs {
				if backendRef.Name == rule.BackendRefs[i].Name {
					rule.BackendRefs[i].Weight = backendRef.Weight
					ruleExists = true
					break
				}
			}
		}

		if !ruleExists {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1.HTTPRouteRule{
				BackendRefs: []gatewayv1.HTTPBackendRef{backendRef},
			})
		}
	}
	if ruleExists {
		notify(fmt.Sprintf("parsed canary annotations of ingress and patched %v fields", field.NewPath("httproute", "spec", "rules").Key("").Child("backendRefs")), httpRoute)
	}
}

func calculateBackendRefWeight(namespace string, servicePorts map[types.NamespacedName]map[string]int32, paths []ingressPath) ([]gatewayv1.HTTPBackendRef, field.ErrorList) {
	var errors field.ErrorList
	var backendRefs []gatewayv1.HTTPBackendRef

	var numWeightedBackends, totalWeightSet int32

	// This is the default value for nginx annotation nginx.ingress.kubernetes.io/canary-weight-total
	var weightTotal = 100

	for i, path := range paths {
		backendRef, err := common.ToBackendRef(namespace, path.path.Backend, servicePorts, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if path.extra != nil && path.extra.canary != nil && path.extra.canary.enable {
			weight := int32(path.extra.canary.weight)
			backendRef.Weight = &weight
			totalWeightSet += weight
			numWeightedBackends++
			if path.extra.canary.weightTotal > 0 {
				weightTotal = path.extra.canary.weightTotal
			}
		}
		backendRefs = append(backendRefs, gatewayv1.HTTPBackendRef{BackendRef: *backendRef})
	}
	if numWeightedBackends > 0 && numWeightedBackends < int32(len(backendRefs)) {
		weightToSet := (int32(weightTotal) - totalWeightSet) / (int32(len(backendRefs)) - numWeightedBackends)
		if weightToSet < 0 {
			weightToSet = 0
		}
		for i := range backendRefs {
			if backendRefs[i].Weight == nil {
				backendRefs[i].Weight = &weightToSet
			}
			if *backendRefs[i].Weight > int32(weightTotal) {
				backendRefs[i].Weight = ptr.To(int32(weightTotal))
			}
		}
	}

	return backendRefs, errors
}

type canaryAnnotations struct {
	enable           bool
	headerKey        string
	headerValue      string
	headerRegexMatch bool
	weight           int
	weightTotal      int
}

func parseCanaryAnnotations(ingress networkingv1.Ingress) (canaryAnnotations, field.ErrorList) {
	var errs field.ErrorList
	var err error

	fieldPath := field.NewPath(ingress.Name).Child("metadata").Child("annotations")

	var annotations canaryAnnotations
	if c := ingress.Annotations["nginx.ingress.kubernetes.io/canary"]; c == "true" {
		annotations.enable = true
		if cHeader := ingress.Annotations["nginx.ingress.kubernetes.io/canary-by-header"]; cHeader != "" {
			annotations.headerKey = cHeader
			annotations.headerValue = "always"
		}
		if cHeaderVal := ingress.Annotations["nginx.ingress.kubernetes.io/canary-by-header-value"]; cHeaderVal != "" {
			annotations.headerValue = cHeaderVal
		}
		if cHeaderRegex := ingress.Annotations["nginx.ingress.kubernetes.io/canary-by-header-pattern"]; cHeaderRegex != "" {
			annotations.headerValue = cHeaderRegex
			annotations.headerRegexMatch = true
		}
		if cHeaderWeight := ingress.Annotations["nginx.ingress.kubernetes.io/canary-weight"]; cHeaderWeight != "" {
			annotations.weight, err = strconv.Atoi(cHeaderWeight)
			if err != nil {
				errs = append(errs, field.TypeInvalid(fieldPath, "nginx.ingress.kubernetes.io/canary-weight", err.Error()))
			}
			annotations.weightTotal = 100
		}
		if cHeaderWeightTotal := ingress.Annotations["nginx.ingress.kubernetes.io/canary-weight-total"]; cHeaderWeightTotal != "" {
			annotations.weightTotal, err = strconv.Atoi(cHeaderWeightTotal)
			if err != nil {
				errs = append(errs, field.TypeInvalid(fieldPath, "nginx.ingress.kubernetes.io/canary-weight-total", err.Error()))
			}
		}
	}
	return annotations, errs
}

func getPathMatchKey(ip ingressPath) pathMatchKey {
	var pathType string
	if ip.path.PathType != nil {
		pathType = string(*ip.path.PathType)
	}
	var canaryHeaderKey string
	if ip.extra != nil && ip.extra.canary != nil && ip.extra.canary.headerKey != "" {
		canaryHeaderKey = ip.extra.canary.headerKey
	}
	return pathMatchKey(fmt.Sprintf("%s/%s/%s", pathType, ip.path.Path, canaryHeaderKey))
}

type pathMatchKey string

type ingressPath struct {
	ingress networkingv1.Ingress

	ruleType string
	path     networkingv1.HTTPIngressPath
	extra    *extra
}

type extra struct {
	canary *canaryAnnotations
}
