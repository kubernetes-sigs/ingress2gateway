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

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func corsFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList
	for key, httpRouteContext := range ir.HTTPRoutes {
		for i := range httpRouteContext.HTTPRoute.Spec.Rules {
			if i >= len(httpRouteContext.RuleBackendSources) {
				continue
			}
			sources := httpRouteContext.RuleBackendSources[i]

			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			// Check if CORS is enabled
			enableCors, ok := ingress.Annotations[EnableCorsAnnotation]
			if !ok || enableCors != "true" {
				continue
			}

			if httpRouteContext.CorsPolicyByRuleIdx == nil {
				httpRouteContext.CorsPolicyByRuleIdx = make(map[int]*gatewayv1.HTTPCORSFilter)
			}

			corsFilter := &gatewayv1.HTTPCORSFilter{}

			// Allow Origin
			if origin, ok := ingress.Annotations[CorsAllowOriginAnnotation]; ok && origin != "" {
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
			if methods, ok := ingress.Annotations[CorsAllowMethodsAnnotation]; ok && methods != "" {
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
			if headers, ok := ingress.Annotations[CorsAllowHeadersAnnotation]; ok && headers != "" {
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
			if exposeHeaders, ok := ingress.Annotations[CorsExposeHeadersAnnotation]; ok && exposeHeaders != "" {
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
			// Nginx default is true. Only false if explicitly set to "false".
			if creds, ok := ingress.Annotations[CorsAllowCredentialsAnnotation]; ok && creds == "false" {
				corsFilter.AllowCredentials = ptr.To(false)
			} else {
				corsFilter.AllowCredentials = ptr.To(true)
			}

			// Max Age
			// Nginx default is 1728000.
			var maxAgeVal int32 = 1728000 // Default from Nginx
			if maxAgeStr, ok := ingress.Annotations[CorsMaxAgeAnnotation]; ok && maxAgeStr != "" {
				// Try parsing as integer (seconds)
				if val, err := strconv.Atoi(maxAgeStr); err == nil {
					maxAgeVal = int32(val)
				} else {
					// Try parsing as duration (e.g. "10s")
					if d, err := time.ParseDuration(maxAgeStr); err == nil {
						maxAgeVal = int32(d.Seconds())
					} else {
						errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations", CorsMaxAgeAnnotation), maxAgeStr, "invalid cors-max-age value"))
					}
				}
			}

			corsFilter.MaxAge = maxAgeVal

			httpRouteContext.CorsPolicyByRuleIdx[i] = corsFilter
		}
		ir.HTTPRoutes[key] = httpRouteContext
	}
	return errs
}
