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
	"errors"
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

type annotationGroup struct {
	provider IngressProvider
	ingress  networkingv1.Ingress

	canaryGroup *canaryGroup
}

func newAnnotationGroup(provider IngressProvider, ingress networkingv1.Ingress) *annotationGroup {
	return &annotationGroup{
		provider:    provider,
		ingress:     ingress,
		canaryGroup: &canaryGroup{},
	}
}

func (a *annotationGroup) retrieve() (err error) {
	a.canaryGroup, err = a.canaryGroup.retrieveAnnotations(a.provider, a.ingress)
	if err != nil {
		return err
	}

	return nil
}

type canaryGroup struct {
	enable           bool
	headerKey        string
	headerValue      string
	headerRegexMatch bool
	weight           int
	weightTotal      int
}

func (c *canaryGroup) retrieveAnnotations(provider IngressProvider, ingress networkingv1.Ingress) (*canaryGroup, error) {
	var (
		cg  = &canaryGroup{}
		err error
	)

	if ingress.Annotations == nil {
		return nil, nil
	}

	switch provider {
	case IngressNginxIngressProvider:
		canary := ingress.Annotations[constructAnnotation(provider, "canary")]
		if canary == "true" {
			cg = &canaryGroup{enable: true}
			if cHeader := ingress.Annotations[constructAnnotation(provider, "canary-by-header")]; cHeader != "" {
				cg.headerKey = cHeader
				cg.headerValue = "always"
			}
			if cHeaderVal := ingress.Annotations[constructAnnotation(provider, "canary-by-header-value")]; cHeaderVal != "" {
				cg.headerValue = cHeaderVal
			}
			if cHeaderRegex := ingress.Annotations[constructAnnotation(provider, "canary-by-header-pattern")]; cHeaderRegex != "" {
				cg.headerValue = cHeaderRegex
				cg.headerRegexMatch = true
			}
			if cHeaderWeight := ingress.Annotations[constructAnnotation(provider, "canary-weight")]; cHeaderWeight != "" {
				cg.weight, err = strconv.Atoi(cHeaderWeight)
				cg.weightTotal = 100
				if err != nil {
					return nil, err
				}
			}
			if cHeaderWeightTotal := ingress.Annotations[constructAnnotation(provider, "canary-weight-total")]; cHeaderWeightTotal != "" {
				cg.weightTotal, err = strconv.Atoi(cHeaderWeightTotal)
				if err != nil {
					return nil, err
				}
			}
		}
	default:
		return nil, errors.New("unsupported ingress provider")
	}

	return cg, nil
}

func constructAnnotation(provider IngressProvider, key string) string {
	if provider == IngressNginxIngressProvider {
		return fmt.Sprintf("%s/%s", string(IngressNginxProviderPrefix), key)
	}

	return ""
}
