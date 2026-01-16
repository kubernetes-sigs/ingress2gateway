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

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func corsFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
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

			// Check if CORS is enabled
			enableCors, ok := ingress.Annotations[EnableCorsAnnotation]
			if !ok || enableCors != "true" {
				continue
			}

			rule := &httpRouteContext.HTTPRoute.Spec.Rules[i]

			// Check if CORS filter already exists (unlikely in this flow, but good practice)
			var corsFilter *gatewayv1.HTTPCORSFilter
			for _, f := range rule.Filters {
				if f.Type == gatewayv1.HTTPRouteFilterCORS && f.CORS != nil {
					corsFilter = f.CORS
					break
				}
			}

			if corsFilter == nil {
				// Create new filter
				newFilter := gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterCORS,
					CORS: &gatewayv1.HTTPCORSFilter{},
				}
				rule.Filters = append(rule.Filters, newFilter)
				corsFilter = rule.Filters[len(rule.Filters)-1].CORS
			}

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
				if val, err := strconv.Atoi(maxAgeStr); err == nil {
					maxAgeVal = int32(val)
				}
			}
			// Gateway API MaxAge is likely specific. Check if MaxAge is a pointer?
			// Diagnostic tool said "Field: MaxAge, Type: int32". So direct assignment.
			// Assuming it supports direct assignment of int32.
			
			// Warning: If MaxAge field DOES NOT exist (e.g. if I am wrong about struct), build fails.
			// I am trusting diagnostic tool output.
			
			corsFilter.MaxAge = maxAgeVal
		}
	}
	return nil
}
