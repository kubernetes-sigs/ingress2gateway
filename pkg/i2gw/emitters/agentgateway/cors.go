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

package agentgateway

import (
	"strings"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyCorsPolicy projects the CORS policy IR into an AgentgatewayPolicy,
// returning true if it modified/created an AgentgatewayPolicy for the given ingress.
//
// This maps ingress-nginx CORS annotations (captured in provider IR) into:
//
//	AgentgatewayPolicy.spec.traffic.cors (which inlines a Gateway API HTTPCORSFilter)
func applyCorsPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.Cors == nil || !pol.Cors.Enable || len(pol.Cors.AllowOrigin) == 0 {
		return false
	}

	// AllowOrigins: dedupe while preserving order.
	seenOrigins := make(map[string]struct{}, len(pol.Cors.AllowOrigin))
	var origins []gatewayv1.CORSOrigin
	for _, o := range pol.Cors.AllowOrigin {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if _, ok := seenOrigins[o]; ok {
			continue
		}
		seenOrigins[o] = struct{}{}
		origins = append(origins, gatewayv1.CORSOrigin(o))
	}
	if len(origins) == 0 {
		return false
	}

	// AllowHeaders: dedupe (case-insensitive) and map to HTTPHeaderName.
	var allowHeaders []gatewayv1.HTTPHeaderName
	if len(pol.Cors.AllowHeaders) > 0 {
		seenHeaders := make(map[string]struct{}, len(pol.Cors.AllowHeaders))
		for _, h := range pol.Cors.AllowHeaders {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			key := strings.ToLower(h)
			if _, ok := seenHeaders[key]; ok {
				continue
			}
			seenHeaders[key] = struct{}{}
			allowHeaders = append(allowHeaders, gatewayv1.HTTPHeaderName(h))
		}
	}

	// ExposeHeaders: dedupe (case-insensitive) and map to HTTPHeaderName.
	var exposeHeaders []gatewayv1.HTTPHeaderName
	if len(pol.Cors.ExposeHeaders) > 0 {
		seenHeaders := make(map[string]struct{}, len(pol.Cors.ExposeHeaders))
		for _, h := range pol.Cors.ExposeHeaders {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			key := strings.ToLower(h)
			if _, ok := seenHeaders[key]; ok {
				continue
			}
			seenHeaders[key] = struct{}{}
			exposeHeaders = append(exposeHeaders, gatewayv1.HTTPHeaderName(h))
		}
	}

	// AllowMethods: normalize to upper-case, filter to Gateway API enum, dedupe.
	var methods []gatewayv1.HTTPMethodWithWildcard
	if len(pol.Cors.AllowMethods) > 0 {
		seenMethods := make(map[string]struct{}, len(pol.Cors.AllowMethods))
		for _, m := range pol.Cors.AllowMethods {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			upper := strings.ToUpper(m)
			if _, ok := seenMethods[upper]; ok {
				continue
			}

			switch upper {
			case "*",
				string(gatewayv1.HTTPMethodGet),
				string(gatewayv1.HTTPMethodHead),
				string(gatewayv1.HTTPMethodPost),
				string(gatewayv1.HTTPMethodPut),
				string(gatewayv1.HTTPMethodDelete),
				string(gatewayv1.HTTPMethodConnect),
				string(gatewayv1.HTTPMethodOptions),
				string(gatewayv1.HTTPMethodTrace),
				string(gatewayv1.HTTPMethodPatch):
				methods = append(methods, gatewayv1.HTTPMethodWithWildcard(upper))
				seenMethods[upper] = struct{}{}
			default:
				// Ignore unsupported method strings to avoid generating invalid objects.
			}
		}
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Traffic == nil {
		agp.Spec.Traffic = &agentgatewayv1alpha1.Traffic{}
	}
	if agp.Spec.Traffic.Cors == nil {
		agp.Spec.Traffic.Cors = &agentgatewayv1alpha1.CORS{}
	}
	if agp.Spec.Traffic.Cors.HTTPCORSFilter == nil {
		agp.Spec.Traffic.Cors.HTTPCORSFilter = &gatewayv1.HTTPCORSFilter{}
	}

	f := agp.Spec.Traffic.Cors.HTTPCORSFilter

	// Required-ish for nginx semantics: we only emit if we have at least one origin.
	f.AllowOrigins = origins

	// Optional knobs: only set when present in the IR.
	if pol.Cors.AllowCredentials != nil {
		f.AllowCredentials = pol.Cors.AllowCredentials
	}
	if len(allowHeaders) > 0 {
		f.AllowHeaders = allowHeaders
	}
	if len(exposeHeaders) > 0 {
		f.ExposeHeaders = exposeHeaders
	}
	if len(methods) > 0 {
		f.AllowMethods = methods
	}
	if pol.Cors.MaxAge != nil && *pol.Cors.MaxAge > 0 {
		f.MaxAge = *pol.Cors.MaxAge
	}

	ap[ingressName] = agp
	return true
}
