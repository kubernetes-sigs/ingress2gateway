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
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var nginxSizeRegex = regexp.MustCompile(`^(\d+)([bkmg])?$`)

// convertNginxSizeToK8sQuantity converts nginx size format to Kubernetes resource.Quantity format.
//
// nginx uses lowercase suffixes for byte sizes:
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
//   - "10m" -> "10M" (10 megabytes)
//   - "10k" -> "10k" (10 kilobytes, same)
//   - "10g" -> "10G" (10 gigabytes)
//   - "100" -> "100" (no unit, same)
func convertNginxSizeToK8sQuantity(nginxSize string) (string, error) {
	nginxSize = strings.TrimSpace(nginxSize)
	nginxSize = strings.ToLower(nginxSize)

	matches := nginxSizeRegex.FindStringSubmatch(nginxSize)
	if matches == nil {
		return "", fmt.Errorf("invalid nginx size format: %q", nginxSize)
	}

	number := matches[1]
	unit := matches[2]

	// Convert nginx unit to K8s Quantity unit
	switch unit {
	case "m":
		return number + "M", nil // megabytes -> Mega
	case "g":
		return number + "G", nil // gigabytes -> Giga
	case "k", "b", "":
		return number + unit, nil // kilobytes, bytes, or no unit stay the same
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
		if !ok {
			continue
		}

		for ruleIdx := range eRouteCtx.Spec.Rules {
			if ruleIdx >= len(pRouteCtx.RuleBackendSources) {
				continue
			}

			var (
				maxSize          *resource.Quantity
				bufferSize       *resource.Quantity
				maxSizeSource    *networkingv1.Ingress
				bufferSizeSource *networkingv1.Ingress
			)

			for _, source := range pRouteCtx.RuleBackendSources[ruleIdx] {
				if source.Ingress.Annotations != nil {
					// handle proxy-body-size
					if val, ok := source.Ingress.Annotations[ProxyBodySizeAnnotation]; ok && val != "" {
						k8sSize, err := convertNginxSizeToK8sQuantity(val)
						if err != nil {
							errs = append(errs, field.Invalid(
								field.NewPath("ingress", source.Ingress.Namespace, source.Ingress.Name, "metadata", "annotations"),
								source.Ingress.Annotations,
								fmt.Sprintf("failed to parse proxy-body-size configuration: %v", err),
							))
							continue
						}

						quantity, err := resource.ParseQuantity(k8sSize)
						if err != nil {
							errs = append(errs, field.Invalid(
								field.NewPath("ingress", source.Ingress.Namespace, source.Ingress.Name, "metadata", "annotations"),
								source.Ingress.Annotations,
								fmt.Sprintf("failed to parse proxy-body-size configuration: %v", err),
							))
							continue
						}

						if maxSize != nil {
							errs = append(errs, field.Invalid(
								field.NewPath("httproute", eRouteCtx.HTTPRoute.Name, "spec", "rules").Index(ruleIdx),
								fmt.Sprintf("ingress %s/%s %s", source.Ingress.Namespace, source.Ingress.Name, ProxyBodySizeAnnotation),
								"at most one proxy-body-size is allowed per rule, this ingress's value is ignored",
							))
							continue
						}
						maxSize = &quantity
						maxSizeSource = source.Ingress
					}

					// handle client-body-buffer-size
					if val, ok := source.Ingress.Annotations[ClientBodyBufferSizeAnnotation]; ok && val != "" {
						k8sSize, err := convertNginxSizeToK8sQuantity(val)
						if err != nil {
							errs = append(errs, field.Invalid(
								field.NewPath("ingress", source.Ingress.Namespace, source.Ingress.Name, "metadata", "annotations"),
								source.Ingress.Annotations,
								fmt.Sprintf("failed to parse client-body-buffer-size configuration: %v", err),
							))
							continue
						}

						quantity, err := resource.ParseQuantity(k8sSize)
						if err != nil {
							errs = append(errs, field.Invalid(
								field.NewPath("ingress", source.Ingress.Namespace, source.Ingress.Name, "metadata", "annotations"),
								source.Ingress.Annotations,
								fmt.Sprintf("failed to parse client-body-buffer-size configuration: %v", err),
							))
							continue
						}

						if bufferSize != nil {
							errs = append(errs, field.Invalid(
								field.NewPath("httproute", eRouteCtx.HTTPRoute.Name, "spec", "rules").Index(ruleIdx),
								fmt.Sprintf("ingress %s/%s %s", source.Ingress.Namespace, source.Ingress.Name, ClientBodyBufferSizeAnnotation),
								"at most one client-body-buffer-size is allowed per rule, this ingress's value is ignored",
							))
							continue
						}
						bufferSize = &quantity
						bufferSizeSource = source.Ingress
					}
				}
			}

			if maxSize == nil && bufferSize == nil {
				continue
			}

			if eRouteCtx.BodySizeByRuleIdx == nil {
				eRouteCtx.BodySizeByRuleIdx = make(map[int]*emitterir.BodySize)
			}

			bodySizeIR := emitterir.BodySize{}
			if maxSize != nil {
				bodySizeIR.MaxSize = maxSize
				notify(notifications.InfoNotification, fmt.Sprintf("parsed proxy-body-size annotation of ingress %s/%s and set %s to HTTPRoute rule index %d",
					maxSizeSource.Namespace, maxSizeSource.Name, maxSize.String(), ruleIdx), &eRouteCtx.HTTPRoute)
			}
			if bufferSize != nil {
				bodySizeIR.BufferSize = bufferSize
				notify(notifications.InfoNotification, fmt.Sprintf("parsed client-body-buffer-size annotation of ingress %s/%s and set %s to HTTPRoute rule index %d",
					bufferSizeSource.Namespace, bufferSizeSource.Name, bufferSize.String(), ruleIdx), &eRouteCtx.HTTPRoute)
			}

			eRouteCtx.BodySizeByRuleIdx[ruleIdx] = &bodySizeIR
		}

		eIR.HTTPRoutes[key] = eRouteCtx
	}
	return errs
}
