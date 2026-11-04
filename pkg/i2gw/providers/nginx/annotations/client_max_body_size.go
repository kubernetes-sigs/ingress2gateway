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

package annotations

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func parseClientMaxBodySize(value string) (*resource.Quantity, error) {
	k8sValue, err := convertNginxSizeToK8sQuantity(value)
	if err != nil {
		return nil, err
	}
	quantity, err := resource.ParseQuantity(k8sValue)
	if err != nil {
		return nil, fmt.Errorf("invalid body size %q: %w", value, err)
	}
	return &quantity, nil
}

// ingressForRule tracks the source of the ingress
func ingressForRule(sources []providerir.BackendSource) *networkingv1.Ingress {
	for _, source := range sources {
		if source.Ingress != nil {
			return source.Ingress
		}
	}
	// source.Ingress is nil
	return nil
}

// BodySizeFeature processes client-max-body-size annotation and converts to IR
func BodySizeFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, pIR *providerir.ProviderIR) field.ErrorList {
	for key, route := range pIR.HTTPRoutes {
		for ruleIdx := range route.HTTPRoute.Spec.Rules {
			if ruleIdx >= len(route.RuleBackendSources) {
				continue
			}

			ingress := ingressForRule(route.RuleBackendSources[ruleIdx])
			if ingress == nil {
				continue
			}

			value := strings.TrimSpace(ingress.Annotations[nginxClientMaxBodySizeAnnotation])
			if value == "" {
				continue
			}

			bodySize := processBodySizeAnnotation(notify, *ingress, value)
			if bodySize == nil {
				continue
			}

			if route.ProviderSpecificIR.BodySizeByRuleIdx == nil {
				route.ProviderSpecificIR.BodySizeByRuleIdx =
					make(map[int]*emitterir.BodySize)
			}

			route.ProviderSpecificIR.BodySizeByRuleIdx[ruleIdx] = bodySize
		}
		pIR.HTTPRoutes[key] = route
	}
	return nil
}

// processBodySizeAnnotation processes the bodySize from BodySizeFeature and returns the emitter BodySize
func processBodySizeAnnotation(notify notifications.NotifyFunc, ingress networkingv1.Ingress, bodySize string) *emitterir.BodySize {
	maxSize, err := parseClientMaxBodySize(bodySize)
	if err != nil {
		notify(
			notifications.ErrorNotification,
			fmt.Sprintf("Invalid %s annotation %q: %v, skipping body size",
				nginxClientMaxBodySizeAnnotation,
				bodySize,
				err,
			),
			&ingress,
		)
		return nil
	}

	annotationPath := field.NewPath(
		ingress.Namespace,
		ingress.Name,
		"metadata",
		"annotations",
		fmt.Sprintf("%q", nginxClientMaxBodySizeAnnotation),
	)

	return &emitterir.BodySize{
		MaxSize: maxSize,
		Metadata: emitterir.NewExtensionFeatureMetadata(
			fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name),
			[]*field.Path{annotationPath},
			"body size default is too large",
		),
	}
}
