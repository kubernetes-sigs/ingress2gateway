/*
Copyright © 2023 Kubernetes Authors

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

package translator

import (
	"fmt"
	"strconv"

	networkingv1 "k8s.io/api/networking/v1"
)

type IngressProvider string

const (
	IngressNginxIngressProvider IngressProvider = "ingress-nginx"
)

type ProviderPrefix string

const (
	IngressNginxProviderPrefix ProviderPrefix = "nginx.ingress.kubernetes.io"
)

type annotations struct {
	canary *canary
}

type canary struct {
	enable           bool
	headerKey        string
	headerValue      string
	headerRegexMatch bool
	weight           int
	weightTotal      int
}

func retrieveAnnotations(provider IngressProvider, ingress networkingv1.Ingress) *annotations {
	anno := &annotations{}
	if provider == IngressNginxIngressProvider {
		if c := ingress.GetAnnotations()[constructAnnotation(provider, "canary")]; c == "true" {
			anno.canary = &canary{enable: true}
			if cHeader := ingress.GetAnnotations()[constructAnnotation(provider, "canary-by-header")]; cHeader != "" {
				anno.canary.headerKey = cHeader
				anno.canary.headerValue = "always"
			}
			if cHeaderVal := ingress.GetAnnotations()[constructAnnotation(provider, "canary-by-header-value")]; cHeaderVal != "" {
				anno.canary.headerValue = cHeaderVal
			}
			if cHeaderRegex := ingress.GetAnnotations()[constructAnnotation(provider, "canary-by-header-pattern")]; cHeaderRegex != "" {
				anno.canary.headerValue = cHeaderRegex
				anno.canary.headerRegexMatch = true
			}
			if cHeaderWeight := ingress.GetAnnotations()[constructAnnotation(provider, "canary-weight")]; cHeaderWeight != "" {
				anno.canary.weight, _ = strconv.Atoi(cHeaderWeight)
				anno.canary.weightTotal = 100
			}
			if cHeaderWeightTotal := ingress.GetAnnotations()[constructAnnotation(provider, "canary-weight-total")]; cHeaderWeightTotal != "" {
				anno.canary.weightTotal, _ = strconv.Atoi(cHeaderWeightTotal)
			}
		}
	}

	return anno
}

func constructAnnotation(provider IngressProvider, key string) string {
	if provider == IngressNginxIngressProvider {
		return fmt.Sprintf("%s/%s", string(IngressNginxProviderPrefix), key)
	}
	return ""
}
