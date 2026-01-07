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
	"strconv"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// durationRegex validates Gateway API Duration format per GEP-2257.
// Valid format: one or more of <digits>(h|m|s|ms) where digits is 1-5 digits.
var durationRegex = regexp.MustCompile(`^([0-9]{1,5}(h|m|s|ms)){1,4}$`)

// timeoutFeature parses timeout annotations from ingress-nginx and applies
// them to HTTPRoute rules as Gateway API HTTPRouteTimeouts.
//
// Timeout mapping rationale:
//   - proxy-read-timeout -> Request timeout (total time for request lifecycle)
//   - proxy-connect-timeout -> BackendRequest timeout (time to establish backend connection)
//
// Note: nginx-ingress proxy-read-timeout is technically the time to read a response
// from the backend, but Gateway API's Request timeout is the closest semantic match
// for controlling overall request duration that users typically care about.
func timeoutFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	for _, httpRouteContext := range ir.HTTPRoutes {
		for i := range httpRouteContext.HTTPRoute.Spec.Rules {
			if i >= len(httpRouteContext.RuleBackendSources) {
				continue
			}
			sources := httpRouteContext.RuleBackendSources[i]

			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			timeouts, parseErrs := parseTimeoutAnnotations(ingress, &httpRouteContext.HTTPRoute)
			errs = append(errs, parseErrs...)

			// Apply valid timeouts even if some annotations had errors
			if timeouts != nil && (timeouts.Request != nil || timeouts.BackendRequest != nil) {
				httpRouteContext.HTTPRoute.Spec.Rules[i].Timeouts = timeouts
				notify(notifications.InfoNotification, fmt.Sprintf("Applied timeout configuration to rule %d of route %s/%s", i, httpRouteContext.HTTPRoute.Namespace, httpRouteContext.HTTPRoute.Name), &httpRouteContext.HTTPRoute)
			}

			// Warn about unsupported annotations
			warnUnsupportedTimeoutAnnotations(ingress, &httpRouteContext.HTTPRoute)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// parseTimeoutAnnotations parses timeout-related annotations and returns HTTPRouteTimeouts.
// It continues parsing valid annotations even if some have errors.
func parseTimeoutAnnotations(ingress *networkingv1.Ingress, httpRoute *gatewayv1.HTTPRoute) (*gatewayv1.HTTPRouteTimeouts, field.ErrorList) {
	var errs field.ErrorList
	var timeouts *gatewayv1.HTTPRouteTimeouts

	// Parse proxy-read-timeout -> Request timeout
	// Maps to the overall request timeout in Gateway API
	if val, ok := ingress.Annotations[ProxyReadTimeoutAnnotation]; ok && val != "" {
		duration, err := parseTimeoutValue(val)
		if err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", ProxyReadTimeoutAnnotation),
				val,
				fmt.Sprintf("invalid timeout value: %v", err),
			))
		} else {
			if timeouts == nil {
				timeouts = &gatewayv1.HTTPRouteTimeouts{}
			}
			timeouts.Request = duration
			notify(notifications.InfoNotification, fmt.Sprintf("Ingress %s/%s: parsed proxy-read-timeout=%s as Request timeout", ingress.Namespace, ingress.Name, val), httpRoute)
		}
	}

	// Parse proxy-connect-timeout -> BackendRequest timeout
	// Maps to the backend connection timeout in Gateway API
	if val, ok := ingress.Annotations[ProxyConnectTimeoutAnnotation]; ok && val != "" {
		duration, err := parseTimeoutValue(val)
		if err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", ProxyConnectTimeoutAnnotation),
				val,
				fmt.Sprintf("invalid timeout value: %v", err),
			))
		} else {
			if timeouts == nil {
				timeouts = &gatewayv1.HTTPRouteTimeouts{}
			}
			timeouts.BackendRequest = duration
			notify(notifications.InfoNotification, fmt.Sprintf("Ingress %s/%s: parsed proxy-connect-timeout=%s as BackendRequest timeout", ingress.Namespace, ingress.Name, val), httpRoute)
		}
	}

	// proxy-send-timeout is logged as info since Gateway API doesn't have a direct equivalent
	if val, ok := ingress.Annotations[ProxySendTimeoutAnnotation]; ok && val != "" {
		notify(notifications.InfoNotification, fmt.Sprintf("Ingress %s/%s: proxy-send-timeout=%s noted (no direct Gateway API equivalent)", ingress.Namespace, ingress.Name, val), httpRoute)
	}

	return timeouts, errs
}

// parseTimeoutValue converts a timeout string (in seconds or with suffix) to Gateway API Duration.
// Accepts:
//   - Plain integers (treated as seconds): "60" -> "60s"
//   - Duration strings with suffix: "30s", "5m", "1h", "500ms"
//   - Combined durations: "1h30m", "1m30s"
func parseTimeoutValue(val string) (*gatewayv1.Duration, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil, fmt.Errorf("empty timeout value")
	}

	// Check if value has a duration suffix (h, m, s, or ms)
	hasDurationSuffix := strings.HasSuffix(val, "s") || strings.HasSuffix(val, "m") || strings.HasSuffix(val, "h")

	if hasDurationSuffix {
		// Validate against Gateway API Duration format per GEP-2257
		if !durationRegex.MatchString(val) {
			return nil, fmt.Errorf("invalid Gateway API duration format: %q (must match pattern like '30s', '5m', '1h', '500ms')", val)
		}
		d := gatewayv1.Duration(val)
		return &d, nil
	}

	// nginx-ingress timeout values are in seconds by default
	seconds, err := strconv.Atoi(val)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q as seconds: %v", val, err)
	}

	if seconds < 0 {
		return nil, fmt.Errorf("timeout cannot be negative: %d", seconds)
	}

	d := gatewayv1.Duration(fmt.Sprintf("%ds", seconds))
	return &d, nil
}

// warnUnsupportedTimeoutAnnotations logs warnings for annotations that don't have
// direct Gateway API equivalents
func warnUnsupportedTimeoutAnnotations(ingress *networkingv1.Ingress, httpRoute *gatewayv1.HTTPRoute) {
	unsupportedAnnotations := map[string]string{
		ProxyNextUpstreamAnnotation:        "proxy-next-upstream (retry policy) is not directly supported in Gateway API HTTPRoute",
		ProxyNextUpstreamTimeoutAnnotation: "proxy-next-upstream-timeout is not directly supported in Gateway API HTTPRoute",
		ProxyNextUpstreamTriesAnnotation:   "proxy-next-upstream-tries is not directly supported in Gateway API HTTPRoute",
		ProxyBodySizeAnnotation:            "proxy-body-size is not directly supported in Gateway API HTTPRoute",
	}

	for annotation, message := range unsupportedAnnotations {
		if val, ok := ingress.Annotations[annotation]; ok && val != "" {
			notify(notifications.WarningNotification, fmt.Sprintf("Ingress %s/%s: %s (value: %s)", ingress.Namespace, ingress.Name, message, val), httpRoute)
		}
	}
}
