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
	"net/http"
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestIngressNGINXCORS(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("typical cors annotations", func(t *testing.T) {
			origin := "https://cors.example.com"
			allowHeaders := "X-Requested-With, Content-Type"
			allowMethods := "GET, POST, OPTIONS"
			exposeHeaders := "X-Expose-1, X-Expose-2"
			maxAge := "600"

			runTestCase(t, &framework.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("cors").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", origin).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-methods", allowMethods).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-headers", allowHeaders).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-credentials", "true").
						WithAnnotation("nginx.ingress.kubernetes.io/cors-max-age", maxAge).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-expose-headers", exposeHeaders).
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"cors": {
						&framework.HTTPRequestVerifier{
							Path:   "/",
							Method: http.MethodOptions,
							AllowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							RequestHeaders: map[string]string{
								"Origin":                         origin,
								"Access-Control-Request-Method":  "POST",
								"Access-Control-Request-Headers": allowHeaders,
							},
							HeaderMatches: []framework.HeaderMatch{
								{
									Name: "Access-Control-Allow-Origin",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
									},
								},
								{
									Name: "Access-Control-Allow-Methods",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i).*GET.*`)},
										{Pattern: regexp.MustCompile(`(?i).*POST.*`)},
										{Pattern: regexp.MustCompile(`(?i).*OPTIONS.*`)},
									},
								},
								{
									Name: "Access-Control-Allow-Headers",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i).*X-Requested-With.*`)},
										{Pattern: regexp.MustCompile(`(?i).*Content-Type.*`)},
									},
								},
								{
									Name: "Access-Control-Allow-Credentials",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
								{
									Name: "Access-Control-Max-Age",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile("^" + maxAge + "$")},
									},
								},
							},
						},
						&framework.HTTPRequestVerifier{
							Path:   "/",
							Method: http.MethodGet,
							RequestHeaders: map[string]string{
								"Origin": origin,
							},
							HeaderMatches: []framework.HeaderMatch{
								{
									Name: "Access-Control-Allow-Origin",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
									},
								},
								{
									Name: "Access-Control-Allow-Credentials",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
								{
									Name: "Access-Control-Expose-Headers",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i).*X-Expose-1.*`)},
										{Pattern: regexp.MustCompile(`(?i).*X-Expose-2.*`)},
									},
								},
							},
						},
					},
				},
			})
		})
		t.Run("cors defaults", func(t *testing.T) {
			origin := "https://cors-defaults.example.com"
			maxAge := "1728000"

			runTestCase(t, &framework.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("cors-defaults").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"cors-defaults": {
						&framework.HTTPRequestVerifier{
							Path:   "/",
							Method: http.MethodOptions,
							AllowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							RequestHeaders: map[string]string{
								"Origin":                         origin,
								"Access-Control-Request-Method":  "POST",
								"Access-Control-Request-Headers": "X-Requested-With",
							},
							HeaderMatches: []framework.HeaderMatch{
								{
									Name: "Access-Control-Allow-Origin",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`^\*$|^` + regexp.QuoteMeta(origin) + `$`)},
									},
								},
								{
									Name: "Access-Control-Allow-Methods",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i).*GET.*`)},
										{Pattern: regexp.MustCompile(`(?i).*PUT.*`)},
										{Pattern: regexp.MustCompile(`(?i).*POST.*`)},
										{Pattern: regexp.MustCompile(`(?i).*DELETE.*`)},
										{Pattern: regexp.MustCompile(`(?i).*PATCH.*`)},
										{Pattern: regexp.MustCompile(`(?i).*OPTIONS.*`)},
									},
								},
								{
									Name: "Access-Control-Allow-Headers",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i).*DNT.*`)},
										{Pattern: regexp.MustCompile(`(?i).*Keep-Alive.*`)},
										{Pattern: regexp.MustCompile(`(?i).*User-Agent.*`)},
										{Pattern: regexp.MustCompile(`(?i).*X-Requested-With.*`)},
										{Pattern: regexp.MustCompile(`(?i).*If-Modified-Since.*`)},
										{Pattern: regexp.MustCompile(`(?i).*Cache-Control.*`)},
										{Pattern: regexp.MustCompile(`(?i).*Content-Type.*`)},
										{Pattern: regexp.MustCompile(`(?i).*Range.*`)},
										{Pattern: regexp.MustCompile(`(?i).*Authorization.*`)},
									},
								},
								{
									Name: "Access-Control-Allow-Credentials",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
								{
									Name: "Access-Control-Max-Age",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile("^" + maxAge + "$")},
									},
								},
							},
						},
						&framework.HTTPRequestVerifier{
							Path:   "/",
							Method: http.MethodGet,
							RequestHeaders: map[string]string{
								"Origin": origin,
							},
							HeaderMatches: []framework.HeaderMatch{
								{
									Name: "Access-Control-Allow-Origin",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`^\*$|^` + regexp.QuoteMeta(origin) + `$`)},
									},
								},
								{
									Name: "Access-Control-Allow-Credentials",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i)^true$`)},
									},
								},
							},
						},
					},
				},
			})
		})
		t.Run("cors denied origin", func(t *testing.T) {
			allowedOrigin := "https://cors-allowed.example.com"
			deniedOrigin := "https://cors-denied.example.com"

			runTestCase(t, &framework.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("cors-denied").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", allowedOrigin).
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"cors-denied": {
						&framework.HTTPRequestVerifier{
							Path:   "/",
							Method: http.MethodOptions,
							AllowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							RequestHeaders: map[string]string{
								"Origin":                         deniedOrigin,
								"Access-Control-Request-Method":  "POST",
								"Access-Control-Request-Headers": "X-Requested-With",
							},
							HeaderAbsent: []string{"Access-Control-Allow-Origin"},
						},
						&framework.HTTPRequestVerifier{
							Path:   "/",
							Method: http.MethodGet,
							RequestHeaders: map[string]string{
								"Origin": deniedOrigin,
							},
							HeaderAbsent: []string{"Access-Control-Allow-Origin"},
						},
					},
				},
			})
		})
		t.Run("cors denied method and header", func(t *testing.T) {
			origin := "https://cors-method.example.com"
			allowedMethods := "GET, POST"
			allowedHeaders := "X-Requested-With"
			deniedMethod := "DELETE"
			deniedHeader := "X-Not-Allowed"

			runTestCase(t, &framework.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("cors-denied-method").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", origin).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-methods", allowedMethods).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-headers", allowedHeaders).
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"cors-denied-method": {
						&framework.HTTPRequestVerifier{
							Path:   "/",
							Method: http.MethodOptions,
							AllowedCodes: []int{
								http.StatusOK,
								http.StatusNoContent,
							},
							RequestHeaders: map[string]string{
								"Origin":                         origin,
								"Access-Control-Request-Method":  deniedMethod,
								"Access-Control-Request-Headers": deniedHeader,
							},
							HeaderMatches: []framework.HeaderMatch{
								{
									Name: "Access-Control-Allow-Origin",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
									},
								},
								{
									Name: "Access-Control-Allow-Methods",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i).*GET.*`)},
										{Pattern: regexp.MustCompile(`(?i).*POST.*`)},
									},
								},
								{
									Name: "Access-Control-Allow-Headers",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i).*` + regexp.QuoteMeta(allowedHeaders) + `.*`)},
									},
								},
								{
									Name: "Access-Control-Allow-Methods",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile(`(?i)` + deniedMethod), Negate: true},
									},
								},
								{
									Name: "Access-Control-Allow-Headers",
									Patterns: []*framework.MaybeNegativePattern{
										{
											Pattern: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(deniedHeader)), Negate: true,
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
