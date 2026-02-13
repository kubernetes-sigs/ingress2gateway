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

package e2e

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestIngressNGINXCORS(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("typical cors annotations", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("cors-%s.com", suffix)
			origin := "https://cors.example.com"
			allowHeaders := "X-Requested-With, Content-Type"
			allowMethods := "GET, POST, OPTIONS"
			exposeHeaders := "X-Expose-1, X-Expose-2"
			maxAge := "600"

			runTestCase(t, &testCase{
				gatewayImplementation:  istio.ProviderName,
				allowExperimentalGWAPI: true,
				providers:              []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("cors").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", origin).
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-methods", allowMethods).
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-headers", allowHeaders).
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-credentials", "true").
						withAnnotation("nginx.ingress.kubernetes.io/cors-max-age", maxAge).
						withAnnotation("nginx.ingress.kubernetes.io/cors-expose-headers", exposeHeaders).
						build(),
				},
				verifiers: map[string][]verifier{
					"cors": {
						&httpRequestVerifier{
							host:   host,
							path:   "/",
							method: http.MethodOptions,
							allowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							requestHeaders: map[string]string{
								"Origin":                         origin,
								"Access-Control-Request-Method":  "POST",
								"Access-Control-Request-Headers": allowHeaders,
							},
							headerMatches: []headerMatch{
								{
									name: "Access-Control-Allow-Origin",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
									},
								},
								{
									name: "Access-Control-Allow-Methods",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i).*GET.*`)},
										{pattern: regexp.MustCompile(`(?i).*POST.*`)},
										{pattern: regexp.MustCompile(`(?i).*OPTIONS.*`)},
									},
								},
								{
									name: "Access-Control-Allow-Headers",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i).*X-Requested-With.*`)},
										{pattern: regexp.MustCompile(`(?i).*Content-Type.*`)},
									},
								},
								{
									name: "Access-Control-Allow-Credentials",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
								{
									name: "Access-Control-Max-Age",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + maxAge + "$")},
									},
								},
							},
						},
						&httpRequestVerifier{
							host:   host,
							path:   "/",
							method: http.MethodGet,
							requestHeaders: map[string]string{
								"Origin": origin,
							},
							headerMatches: []headerMatch{
								{
									name: "Access-Control-Allow-Origin",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
									},
								},
								{
									name: "Access-Control-Allow-Credentials",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
								{
									name: "Access-Control-Expose-Headers",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i).*X-Expose-1.*`)},
										{pattern: regexp.MustCompile(`(?i).*X-Expose-2.*`)},
									},
								},
							},
						},
					},
				},
			})
		})
		t.Run("cors defaults", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("cors-defaults-%s.com", suffix)
			origin := "https://cors-defaults.example.com"
			maxAge := "1728000"

			runTestCase(t, &testCase{
				gatewayImplementation:  istio.ProviderName,
				allowExperimentalGWAPI: true,
				providers:              []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("cors-defaults").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						build(),
				},
				verifiers: map[string][]verifier{
					"cors-defaults": {
						&httpRequestVerifier{
							host:   host,
							path:   "/",
							method: http.MethodOptions,
							allowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							requestHeaders: map[string]string{
								"Origin":                         origin,
								"Access-Control-Request-Method":  "POST",
								"Access-Control-Request-Headers": "X-Requested-With",
							},
							headerMatches: []headerMatch{
								{
									name: "Access-Control-Allow-Origin",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`^\*$|^` + regexp.QuoteMeta(origin) + `$`)},
									},
								},
								{
									name: "Access-Control-Allow-Methods",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i).*GET.*`)},
										{pattern: regexp.MustCompile(`(?i).*PUT.*`)},
										{pattern: regexp.MustCompile(`(?i).*POST.*`)},
										{pattern: regexp.MustCompile(`(?i).*DELETE.*`)},
										{pattern: regexp.MustCompile(`(?i).*PATCH.*`)},
										{pattern: regexp.MustCompile(`(?i).*OPTIONS.*`)},
									},
								},
								{
									name: "Access-Control-Allow-Headers",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i).*DNT.*`)},
										{pattern: regexp.MustCompile(`(?i).*Keep-Alive.*`)},
										{pattern: regexp.MustCompile(`(?i).*User-Agent.*`)},
										{pattern: regexp.MustCompile(`(?i).*X-Requested-With.*`)},
										{pattern: regexp.MustCompile(`(?i).*If-Modified-Since.*`)},
										{pattern: regexp.MustCompile(`(?i).*Cache-Control.*`)},
										{pattern: regexp.MustCompile(`(?i).*Content-Type.*`)},
										{pattern: regexp.MustCompile(`(?i).*Range.*`)},
										{pattern: regexp.MustCompile(`(?i).*Authorization.*`)},
									},
								},
								{
									name: "Access-Control-Allow-Credentials",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
								{
									name: "Access-Control-Max-Age",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + maxAge + "$")},
									},
								},
							},
						},
						&httpRequestVerifier{
							host:   host,
							path:   "/",
							method: http.MethodGet,
							requestHeaders: map[string]string{
								"Origin": origin,
							},
							headerMatches: []headerMatch{
								{
									name: "Access-Control-Allow-Origin",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`^\*$|^` + regexp.QuoteMeta(origin) + `$`)},
									},
								},
								{
									name: "Access-Control-Allow-Credentials",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
							},
						},
					},
				},
			})
		})
		t.Run("cors denied origin", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("cors-denied-%s.com", suffix)
			allowedOrigin := "https://cors-allowed.example.com"
			deniedOrigin := "https://cors-denied.example.com"

			runTestCase(t, &testCase{
				gatewayImplementation:  istio.ProviderName,
				allowExperimentalGWAPI: true,
				providers:              []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("cors-denied").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", allowedOrigin).
						build(),
				},
				verifiers: map[string][]verifier{
					"cors-denied": {
						&httpRequestVerifier{
							host:   host,
							path:   "/",
							method: http.MethodOptions,
							allowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							requestHeaders: map[string]string{
								"Origin":                         deniedOrigin,
								"Access-Control-Request-Method":  "POST",
								"Access-Control-Request-Headers": "X-Requested-With",
							},
							headerAbsent: []string{"Access-Control-Allow-Origin"},
						},
						&httpRequestVerifier{
							host:   host,
							path:   "/",
							method: http.MethodGet,
							requestHeaders: map[string]string{
								"Origin": deniedOrigin,
							},
							headerAbsent: []string{"Access-Control-Allow-Origin"},
						},
					},
				},
			})
		})
		t.Run("cors denied method and header", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("cors-denied-method-%s.com", suffix)
			origin := "https://cors-method.example.com"
			allowedMethods := "GET, POST"
			allowedHeaders := "X-Requested-With"
			deniedMethod := "DELETE"
			deniedHeader := "X-Not-Allowed"

			runTestCase(t, &testCase{
				gatewayImplementation:  istio.ProviderName,
				allowExperimentalGWAPI: true,
				providers:              []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("cors-denied-method").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", origin).
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-methods", allowedMethods).
						withAnnotation("nginx.ingress.kubernetes.io/cors-allow-headers", allowedHeaders).
						build(),
				},
				verifiers: map[string][]verifier{
					"cors-denied-method": {
						&httpRequestVerifier{
							host:   host,
							path:   "/",
							method: http.MethodOptions,
							allowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							requestHeaders: map[string]string{
								"Origin":                         origin,
								"Access-Control-Request-Method":  deniedMethod,
								"Access-Control-Request-Headers": deniedHeader,
							},
							headerMatches: []headerMatch{
								{
									name: "Access-Control-Allow-Origin",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
									},
								},
								{
									name: "Access-Control-Allow-Methods",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i).*GET.*`)},
										{pattern: regexp.MustCompile(`(?i).*POST.*`)},
									},
								},
								{
									name: "Access-Control-Allow-Headers",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i).*` + regexp.QuoteMeta(allowedHeaders) + `.*`)},
									},
								},
								{
									name: "Access-Control-Allow-Methods",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile(`(?i)` + deniedMethod), negate: true},
									},
								},
								{
									name: "Access-Control-Allow-Headers",
									patterns: []*maybeNegativePattern{
										{
											pattern: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(deniedHeader)), negate: true,
										},
									},
								},
							},
						},
					},
				},
			})
		})
	})
}
