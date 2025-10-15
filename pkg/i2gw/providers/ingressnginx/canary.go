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
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)



type canaryAnnotations struct {
	enable           bool
	headerKey        string
	headerValue      string
	headerRegexMatch bool
	weight           int
	weightTotal      int
}

func parseCanaryAnnotations(ingress *networkingv1.Ingress) (canaryAnnotations, field.ErrorList) {
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

type ingressPathWithCanary struct {
	path   *i2gw.IngressPath
	canary *canaryAnnotations
}
