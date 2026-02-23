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
	"fmt"
	"regexp"
	"strings"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Ref: https://github.com/kubernetes/ingress-nginx/blob/main/internal/ingress/annotations/parser/validators.go#L57
var nginxSizeRegex = regexp.MustCompile(`^(?i)(\d+)([bkmg]?)$`)

// convertNginxSizeToK8sQuantity converts nginx size format to Kubernetes resource.Quantity format.
//
// nginx uses lowercase suffixes for byte sizes:
//   - b = bytes
//   - k = kilobytes (10^3)
//   - m = megabytes (10^6)
//   - g = gigabytes (10^9)
//
// Kubernetes resource.Quantity uses different suffixes:
//   - m = milli (10^-3)
//   - k = kilo (10^3)
//   - M = mega (10^6)
//   - G = giga (10^9)
//
// This function converts nginx format to K8s format:
//   - "10b" -> "10" (10 bytes)
//   - "10k" -> "10k" (10 kilobytes, same)
//   - "10m" -> "10M" (10 megabytes)
//   - "10g" -> "10G" (10 gigabytes)
//   - "100" -> "100" (no unit, same)
func convertNginxSizeToK8sQuantity(nginxSize string) (string, error) {
	nginxSize = strings.TrimSpace(nginxSize)

	matches := nginxSizeRegex.FindStringSubmatch(nginxSize)
	if matches == nil {
		return "", fmt.Errorf("invalid nginx size format: %q", nginxSize)
	}

	number := matches[1]
	unit := matches[2]

	// Convert nginx unit to K8s Quantity unit
	switch unit {
	case "b", "":
		return number, nil
	case "k":
		return number + "k", nil
	case "m":
		return number + "M", nil
	case "g":
		return number + "G", nil
	default:
		return "", fmt.Errorf("unsupported nginx size unit: %q", unit)
	}
}

// applyBodySizeToEmitterIR reads ingress-nginx body size annotations from ProviderIR sources and stores
// provider-neutral body size intent into EmitterIR, which will later be converted by each custom emitter.
//
// Currently supported annotations are:
// - nginx.ingress.kubernetes.io/proxy-body-size
// - nginx.ingress.kubernetes.io/client-body-buffer-size
func applyBodySizeToEmitterIR(pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) field.ErrorList {
	var errs field.ErrorList

	for key, pRouteCtx := range pIR.HTTPRoutes {
		eRouteCtx, ok := eIR.HTTPRoutes[key]
		parsedAnnotations := make([]string, 2)
		if !ok {
			continue
		}

		for ruleIdx := range eRouteCtx.Spec.Rules {
			if ruleIdx >= len(pRouteCtx.RuleBackendSources) {
				continue
			}
			ing := getNonCanaryIngress(pRouteCtx.RuleBackendSources[ruleIdx])
			if ing == nil {
				continue
			}

			var (
				maxSize    *resource.Quantity
				bufferSize *resource.Quantity
			)

			// handle proxy-body-size
			if val, ok := ing.Annotations[ProxyBodySizeAnnotation]; ok && val != "" {
				parsedAnnotations = append(parsedAnnotations, ProxyBodySizeAnnotation)
				k8sSize, err := convertNginxSizeToK8sQuantity(val)
				if err != nil {
					errs = append(errs, field.Invalid(
						field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations"),
						ing.Annotations,
						fmt.Sprintf("failed to parse proxy-body-size configuration: %v", err),
					))
					continue
				}

				quantity, err := resource.ParseQuantity(k8sSize)
				if err != nil {
					errs = append(errs, field.Invalid(
						field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations"),
						ing.Annotations,
						fmt.Sprintf("failed to parse proxy-body-size configuration: %v", err),
					))
					continue
				}
				maxSize = &quantity
			}

			// handle client-body-buffer-size
			if val, ok := ing.Annotations[ClientBodyBufferSizeAnnotation]; ok && val != "" {
				parsedAnnotations = append(parsedAnnotations, ClientBodyBufferSizeAnnotation)
				k8sSize, err := convertNginxSizeToK8sQuantity(val)
				if err != nil {
					errs = append(errs, field.Invalid(
						field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations"),
						ing.Annotations,
						fmt.Sprintf("failed to parse client-body-buffer-size configuration: %v", err),
					))
					continue
				}

				quantity, err := resource.ParseQuantity(k8sSize)
				if err != nil {
					errs = append(errs, field.Invalid(
						field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations"),
						ing.Annotations,
						fmt.Sprintf("failed to parse client-body-buffer-size configuration: %v", err),
					))
					continue
				}
				bufferSize = &quantity
			}

			if maxSize == nil && bufferSize == nil {
				continue
			}

			if eRouteCtx.BodySizeByRuleIdx == nil {
				eRouteCtx.BodySizeByRuleIdx = make(map[int]*emitterir.BodySize)
			}

			bodySizeIR := emitterir.BodySize{}
			{
				source := fmt.Sprintf("%s/%s", ing.Name, ing.Namespace)
				message := "Most Gateway API implementations have reasonable body size and buffering defaults"
				paths := make([]*field.Path, len(parsedAnnotations))
				for i, ann := range parsedAnnotations {
					paths[i] = field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations", ann)
				}
				bodySizeIR.Metadata = emitterir.NewExtensionFeatureMetadata(
					source,
					paths,
					message,
				)
			}

			if maxSize != nil {
				bodySizeIR.MaxSize = maxSize
				notify(notifications.InfoNotification, fmt.Sprintf("parsed proxy-body-size annotation of ingress %s/%s and set %s to HTTPRoute rule index %d",
					ing.Namespace, ing.Name, maxSize.String(), ruleIdx), &eRouteCtx.HTTPRoute)
			}
			if bufferSize != nil {
				bodySizeIR.BufferSize = bufferSize
				notify(notifications.InfoNotification, fmt.Sprintf("parsed client-body-buffer-size annotation of ingress %s/%s and set %s to HTTPRoute rule index %d",
					ing.Namespace, ing.Name, bufferSize.String(), ruleIdx), &eRouteCtx.HTTPRoute)
			}

			eRouteCtx.BodySizeByRuleIdx[ruleIdx] = &bodySizeIR
		}

		eIR.HTTPRoutes[key] = eRouteCtx
	}
	return errs
}
