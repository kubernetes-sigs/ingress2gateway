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

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyCorsToEmitterIR parses CORS annotations and populates the EmitterIR.
// It matches the pattern of applyRewriteTargetToEmitterIR by applying changes directly to EmitterIR
// after the initial ProviderIR -> EmitterIR conversion.
func applyCorsToEmitterIR(pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) field.ErrorList {
	var errs field.ErrorList
	for key, pRouteCtx := range pIR.HTTPRoutes {
		eRouteCtx, ok := eIR.HTTPRoutes[key]
		if !ok {
			continue
		}

		for ruleIdx := range pRouteCtx.HTTPRoute.Spec.Rules {
			if ruleIdx >= len(pRouteCtx.RuleBackendSources) {
				continue
			}
			sources := pRouteCtx.RuleBackendSources[ruleIdx]

			ing := getNonCanaryIngress(sources)
			if ing == nil {
				continue
			}

			// Check if CORS is enabled
			enableCors, ok := ing.Annotations[EnableCorsAnnotation]
			if !ok {
				continue
			}
			if enabled, err := strconv.ParseBool(enableCors); err != nil || !enabled {
				continue
			}

			if eRouteCtx.CorsPolicyByRuleIdx == nil {
				eRouteCtx.CorsPolicyByRuleIdx = make(map[int]*emitterir.CORSConfig)
			}

			corsFilter := &emitterir.CORSConfig{}

			// Allow Origin
			if origin, ok := ing.Annotations[CorsAllowOriginAnnotation]; ok && origin != "" {
				origins := strings.Split(origin, ",")
				for _, o := range origins {
					o = strings.TrimSpace(o)
					if o == "" {
						continue
					}
					corsFilter.AllowOrigins = append(corsFilter.AllowOrigins, gatewayv1.CORSOrigin(o))
				}
			} else {
				// Default to *
				corsFilter.AllowOrigins = []gatewayv1.CORSOrigin{"*"}
			}

			// Allow Methods
			if methods, ok := ing.Annotations[CorsAllowMethodsAnnotation]; ok && methods != "" {
				methodList := strings.Split(methods, ",")
				for _, m := range methodList {
					m = strings.TrimSpace(m)
					if m == "" {
						continue
					}
					corsFilter.AllowMethods = append(corsFilter.AllowMethods, gatewayv1.HTTPMethodWithWildcard(m))
				}
			} else {
				// Default methods: GET, PUT, POST, DELETE, PATCH, OPTIONS
				corsFilter.AllowMethods = []gatewayv1.HTTPMethodWithWildcard{"GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"}
			}

			// Allow Headers
			if headers, ok := ing.Annotations[CorsAllowHeadersAnnotation]; ok && headers != "" {
				headerList := strings.Split(headers, ",")
				for _, h := range headerList {
					h = strings.TrimSpace(h)
					if h == "" {
						continue
					}
					corsFilter.AllowHeaders = append(corsFilter.AllowHeaders, gatewayv1.HTTPHeaderName(h))
				}
			} else {
				// Default headers from Nginx documentation
				defaultHeaders := []string{"DNT", "Keep-Alive", "User-Agent", "X-Requested-With", "If-Modified-Since", "Cache-Control", "Content-Type", "Range", "Authorization"}
				for _, h := range defaultHeaders {
					corsFilter.AllowHeaders = append(corsFilter.AllowHeaders, gatewayv1.HTTPHeaderName(h))
				}
			}

			// Expose Headers
			if exposeHeaders, ok := ing.Annotations[CorsExposeHeadersAnnotation]; ok && exposeHeaders != "" {
				headerList := strings.Split(exposeHeaders, ",")
				for _, h := range headerList {
					h = strings.TrimSpace(h)
					if h == "" {
						continue
					}
					corsFilter.ExposeHeaders = append(corsFilter.ExposeHeaders, gatewayv1.HTTPHeaderName(h))
				}
			}

			// Allow Credentials
			// Nginx default is true. Only false if explicitly set to "false" (or other falsy values).
			corsFilter.AllowCredentials = ptr.To(true)
			if creds, ok := ing.Annotations[CorsAllowCredentialsAnnotation]; ok {
				if allowed, err := strconv.ParseBool(creds); err == nil && !allowed {
					corsFilter.AllowCredentials = ptr.To(false)
				}
			}

			// Max Age
			// Nginx default is 1728000.
			var maxAgeVal int32 = 1728000 // Default from Nginx
			if maxAgeStr, ok := ing.Annotations[CorsMaxAgeAnnotation]; ok && maxAgeStr != "" {
				// Try parsing as integer (seconds) using ParseInt with bitSize 32
				if val, err := strconv.ParseInt(maxAgeStr, 10, 32); err == nil {
					maxAgeVal = int32(val)
				} else {
					errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations", CorsMaxAgeAnnotation), maxAgeStr, "invalid cors-max-age value"))
				}
			}

			corsFilter.MaxAge = maxAgeVal

			eRouteCtx.CorsPolicyByRuleIdx[ruleIdx] = corsFilter
		}
		eIR.HTTPRoutes[key] = eRouteCtx
	}
	return errs
}
