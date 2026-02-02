/*
Copyright 2026 The Kubernetes Authors.

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
	"strconv"

	networkingv1 "k8s.io/api/networking/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// parseBoolAnnotation checks if an ingress has a specific annotation set to true.
// It uses strconv.ParseBool to handle various boolean representations (true, 1, t, TRUE, etc.).
func parseBoolAnnotation(ingress *networkingv1.Ingress, annotationKey string) bool {
	if ingress == nil || ingress.Annotations == nil {
		return false
	}
	if value, ok := ingress.Annotations[annotationKey]; ok {
		if boolValue, err := strconv.ParseBool(value); err == nil && boolValue {
			return true
		}
	}
	return false
}

func hasUseRegex(ingress *networkingv1.Ingress) bool {
	return parseBoolAnnotation(ingress, UseRegexAnnotation)
}

func isCanary(ingress *networkingv1.Ingress) bool {
	return parseBoolAnnotation(ingress, CanaryAnnotation)
}

// implementationSpecificHTTPPathTypeMatch handles the ImplementationSpecific path type
// for ingress-nginx. When ImplementationSpecific is used, ingress-nginx treats paths
// as regular expressions only if the annotation "nginx.ingress.kubernetes.io/use-regex"
// is present and parses to true. Otherwise, it defaults to Prefix matching.
//
// For canary ingresses, annotations are inherited from the main ingress. This function
// automatically selects the non-canary ingress from the list to determine the path type,
// ensuring that canaries inherit the use-regex setting from their main ingress.
func implementationSpecificHTTPPathTypeMatch(path *gatewayv1.HTTPPathMatch, ingresses []networkingv1.Ingress) {
	pmPrefix := gatewayv1.PathMatchPathPrefix
	pmRegex := gatewayv1.PathMatchRegularExpression

	// Find the non-canary ingress to check for use-regex annotation
	for i := range ingresses {
		if !isCanary(&ingresses[i]) {
			if hasUseRegex(&ingresses[i]) {
				path.Type = &pmRegex
				return
			}
			path.Type = &pmPrefix
			return
		}
	}

	// If all ingresses are canaries (edge case), default to prefix
	path.Type = &pmPrefix
}
