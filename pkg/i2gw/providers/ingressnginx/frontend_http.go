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
	"strings"
	"time"

	providerir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	frontendHTTP1MaxHeadersAnnotation           = "nginx.ingress.kubernetes.io/http1-max-headers"
	frontendHTTP1IdleTimeoutAnnotation          = "nginx.ingress.kubernetes.io/http1-idle-timeout"
	frontendHTTP2WindowSizeAnnotation           = "nginx.ingress.kubernetes.io/http2-window-size"
	frontendHTTP2ConnectionWindowSizeAnnotation = "nginx.ingress.kubernetes.io/http2-connection-window-size"
	frontendHTTP2FrameSizeAnnotation            = "nginx.ingress.kubernetes.io/http2-frame-size"
	frontendHTTP2KeepaliveIntervalAnnotation    = "nginx.ingress.kubernetes.io/http2-keepalive-interval"
	frontendHTTP2KeepaliveTimeoutAnnotation     = "nginx.ingress.kubernetes.io/http2-keepalive-timeout"
)

// frontendHTTPFeature parses frontend HTTP listener settings from ingress-nginx annotations and
// records them into ingress-nginx policy IR.
func frontendHTTPFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *providerir.ProviderIR,
) field.ErrorList {
	var errs field.ErrorList
	perIngress := map[types.NamespacedName]*providerir.IngressNginxFrontendHTTPPolicy{}

	for i := range ingresses {
		ing := &ingresses[i]
		policy := &providerir.IngressNginxFrontendHTTPPolicy{}
		added := false

		if v, ok := parsePositiveInt32Annotation(ing, frontendHTTP1MaxHeadersAnnotation, &errs); ok {
			policy.HTTP1MaxHeaders = v
			added = true
		}
		if v, ok := parseDurationAnnotation(ing, frontendHTTP1IdleTimeoutAnnotation, time.Second, &errs); ok {
			policy.HTTP1IdleTimeout = v
			added = true
		}
		if v, ok := parsePositiveInt32Annotation(ing, frontendHTTP2WindowSizeAnnotation, &errs); ok {
			policy.HTTP2WindowSize = v
			added = true
		}
		if v, ok := parsePositiveInt32Annotation(ing, frontendHTTP2ConnectionWindowSizeAnnotation, &errs); ok {
			policy.HTTP2ConnectionWindowSize = v
			added = true
		}
		if v, ok := parsePositiveInt32Annotation(ing, frontendHTTP2FrameSizeAnnotation, &errs); ok {
			policy.HTTP2FrameSize = v
			added = true
		}
		if v, ok := parseDurationAnnotation(ing, frontendHTTP2KeepaliveIntervalAnnotation, time.Second, &errs); ok {
			policy.HTTP2KeepaliveInterval = v
			added = true
		}
		if v, ok := parseDurationAnnotation(ing, frontendHTTP2KeepaliveTimeoutAnnotation, time.Second, &errs); ok {
			policy.HTTP2KeepaliveTimeout = v
			added = true
		}

		if !added {
			continue
		}

		key := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
		perIngress[key] = policy
	}

	if len(perIngress) == 0 {
		return errs
	}

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		routeKey := types.NamespacedName{
			Namespace: rg.Namespace,
			Name:      common.RouteName(rg.Name, rg.Host),
		}

		httpCtx, ok := ir.HTTPRoutes[routeKey]
		if !ok {
			continue
		}

		for ruleIdx, backendSources := range httpCtx.RuleBackendSources {
			for backendIdx, src := range backendSources {
				if src.Ingress == nil {
					continue
				}

				ingKey := types.NamespacedName{Namespace: src.Ingress.Namespace, Name: src.Ingress.Name}
				frontendHTTP := perIngress[ingKey]
				if frontendHTTP == nil {
					continue
				}

				if httpCtx.ProviderSpecificIR.IngressNginx == nil {
					httpCtx.ProviderSpecificIR.IngressNginx = &providerir.IngressNginxHTTPRouteIR{Policies: map[string]providerir.Policy{}}
				} else if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
					httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]providerir.Policy{}
				}

				existing := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if existing.FrontendHTTP == nil {
					existing.FrontendHTTP = &providerir.IngressNginxFrontendHTTPPolicy{}
				}

				mergeFrontendHTTPPolicy(existing.FrontendHTTP, frontendHTTP)
				existing = existing.AddRuleBackendSources([]providerir.PolicyIndex{{Rule: ruleIdx, Backend: backendIdx}})
				httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = existing
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}

func mergeFrontendHTTPPolicy(dst, src *providerir.IngressNginxFrontendHTTPPolicy) {
	if dst.HTTP1MaxHeaders == nil {
		dst.HTTP1MaxHeaders = src.HTTP1MaxHeaders
	}
	if dst.HTTP1IdleTimeout == nil {
		dst.HTTP1IdleTimeout = src.HTTP1IdleTimeout
	}
	if dst.HTTP2WindowSize == nil {
		dst.HTTP2WindowSize = src.HTTP2WindowSize
	}
	if dst.HTTP2ConnectionWindowSize == nil {
		dst.HTTP2ConnectionWindowSize = src.HTTP2ConnectionWindowSize
	}
	if dst.HTTP2FrameSize == nil {
		dst.HTTP2FrameSize = src.HTTP2FrameSize
	}
	if dst.HTTP2KeepaliveInterval == nil {
		dst.HTTP2KeepaliveInterval = src.HTTP2KeepaliveInterval
	}
	if dst.HTTP2KeepaliveTimeout == nil {
		dst.HTTP2KeepaliveTimeout = src.HTTP2KeepaliveTimeout
	}
}

func parsePositiveInt32Annotation(
	ing *networkingv1.Ingress,
	annotation string,
	errs *field.ErrorList,
) (*int32, bool) {
	raw := strings.TrimSpace(ing.Annotations[annotation])
	if raw == "" {
		return nil, false
	}

	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || parsed <= 0 {
		*errs = append(*errs, field.Invalid(
			field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(annotation),
			raw,
			"must be a positive integer",
		))
		return nil, false
	}

	value := int32(parsed)
	return &value, true
}

func parseDurationAnnotation(
	ing *networkingv1.Ingress,
	annotation string,
	min time.Duration,
	errs *field.ErrorList,
) (*metav1.Duration, bool) {
	raw := strings.TrimSpace(ing.Annotations[annotation])
	if raw == "" {
		return nil, false
	}

	duration, err := time.ParseDuration(raw)
	if err != nil {
		duration, err = time.ParseDuration(raw + "s")
	}
	if err != nil {
		*errs = append(*errs, field.Invalid(
			field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(annotation),
			raw,
			"must be a valid duration (for example 5s or 1m)",
		))
		return nil, false
	}
	if duration < min {
		*errs = append(*errs, field.Invalid(
			field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(annotation),
			raw,
			"must be at least 1s",
		))
		return nil, false
	}

	return &metav1.Duration{Duration: duration}, true
}
