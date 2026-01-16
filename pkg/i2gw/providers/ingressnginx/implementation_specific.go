/*
Copyright 2025 The Kubernetes Authors.

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

const (
	useRegexAnnotation = "nginx.ingress.kubernetes.io/use-regex"
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
	return parseBoolAnnotation(ingress, useRegexAnnotation)
}

func isCanary(ingress *networkingv1.Ingress) bool {
	return parseBoolAnnotation(ingress, canaryAnnotation)
}

// selectRepresentativeIngress selects the most appropriate ingress from a list
// of ingresses that share the same path. For ingress-nginx, canary ingresses
// inherit annotations from their main (non-canary) ingress. Therefore, we should
// always select the main ingress for path matching, as the canary will inherit
// its use-regex setting.
//
// According to NGINX Ingress documentation:
// "when you mark an ingress as canary, then all the other non-canary annotations
// will be ignored (inherited from the corresponding main ingress)"
func selectRepresentativeIngress(ingresses []*networkingv1.Ingress) *networkingv1.Ingress {
	if len(ingresses) == 0 {
		return nil
	}

	for _, ing := range ingresses {
		if !isCanary(ing) {
			return ing
		}
	}
	return ingresses[0]
}

// implementationSpecificHTTPPathTypeMatch handles the ImplementationSpecific path type
// for ingress-nginx. When ImplementationSpecific is used, ingress-nginx treats paths
// as regular expressions only if the annotation "nginx.ingress.kubernetes.io/use-regex"
// is present and parses to true. Otherwise, it defaults to Prefix matching.
//
// Note: For canary ingresses, annotations are inherited from the main ingress.
// The selectRepresentativeIngress function ensures that the main (non-canary) ingress
// is passed to this function, so canaries automatically inherit the use-regex setting.
func implementationSpecificHTTPPathTypeMatch(path *gatewayv1.HTTPPathMatch, ingress *networkingv1.Ingress) {
	pmPrefix := gatewayv1.PathMatchPathPrefix
	pmRegex := gatewayv1.PathMatchRegularExpression

	if hasUseRegex(ingress) {
		path.Type = &pmRegex
		return
	}
	path.Type = &pmPrefix
}
