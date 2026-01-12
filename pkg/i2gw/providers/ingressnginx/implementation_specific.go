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
	networkingv1 "k8s.io/api/networking/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	useRegexAnnotation = "nginx.ingress.kubernetes.io/use-regex"
)

// implementationSpecificHTTPPathTypeMatch handles the ImplementationSpecific path type
// for ingress-nginx. When ImplementationSpecific is used, ingress-nginx treats paths
// as regular expressions only if the annotation "nginx.ingress.kubernetes.io/use-regex: true"
// is present. Otherwise, it defaults to Prefix matching.
func implementationSpecificHTTPPathTypeMatch(path *gatewayv1.HTTPPathMatch, ingress *networkingv1.Ingress) {
	pmPrefix := gatewayv1.PathMatchPathPrefix
	pmRegex := gatewayv1.PathMatchRegularExpression

	// Check if use-regex annotation is set to "true"
	if ingress != nil && ingress.Annotations != nil {
		if useRegex, ok := ingress.Annotations[useRegexAnnotation]; ok && useRegex == "true" {
			path.Type = &pmRegex
			return
		}
	}

	// Default to Prefix matching if use-regex is not set or not "true"
	path.Type = &pmPrefix
}
