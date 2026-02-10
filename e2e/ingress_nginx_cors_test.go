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

func TestCORS(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("typical cors annotations", func(t *testing.T) {
			suffix, err := e2e.RandString(6)
			require.NoError(t, err)
			host := fmt.Sprintf("cors-%s.com", suffix)
			origin := "https://cors.example.com"
			allowHeaders := "X-Requested-With, Content-Type"
			allowMethods := "GET, POST, OPTIONS"
			exposeHeaders := "X-Expose-1, X-Expose-2"
			maxAge := "600"

			e2e.RunTestCase(t, &e2e.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					e2e.BasicIngress().
						WithName("cors").
						WithHost(host).
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
				Verifiers: map[string][]e2e.Verifier{
					"cors": {
						&e2e.HttpGetVerifier{
							Host:   host,
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
							HeaderMatches: []e2e.HeaderMatch{
								{
									Name:     "Access-Control-Allow-Origin",
									Patterns: []*regexp.Regexp{regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
								},
								{
									Name: "Access-Control-Allow-Methods",
									Patterns: []*regexp.Regexp{
										regexp.MustCompile(`(?i).*GET.*`),
										regexp.MustCompile(`(?i).*POST.*`),
										regexp.MustCompile(`(?i).*OPTIONS.*`),
									},
								},
								{
									Name: "Access-Control-Allow-Headers",
									Patterns: []*regexp.Regexp{
										regexp.MustCompile(`(?i).*X-Requested-With.*`),
										regexp.MustCompile(`(?i).*Content-Type.*`),
									},
								},
								{
									Name:     "Access-Control-Allow-Credentials",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^true$`)},
								},
								{
									Name:     "Access-Control-Max-Age",
									Patterns: []*regexp.Regexp{regexp.MustCompile("^" + maxAge + "$")},
								},
							},
						},
						&e2e.HttpGetVerifier{
							Host:   host,
							Path:   "/",
							Method: http.MethodGet,
							RequestHeaders: map[string]string{
								"Origin": origin,
							},
							HeaderMatches: []e2e.HeaderMatch{
								{
									Name:     "Access-Control-Allow-Origin",
									Patterns: []*regexp.Regexp{regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
								},
								{
									Name:     "Access-Control-Allow-Credentials",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^true$`)},
								},
								{
									Name: "Access-Control-Expose-Headers",
									Patterns: []*regexp.Regexp{
										regexp.MustCompile(`(?i).*X-Expose-1.*`),
										regexp.MustCompile(`(?i).*X-Expose-2.*`),
									},
								},
							},
						},
					},
				},
			})
		})
		t.Run("cors defaults", func(t *testing.T) {
			suffix, err := e2e.RandString(6)
			require.NoError(t, err)
			host := fmt.Sprintf("cors-defaults-%s.com", suffix)
			origin := "https://cors-defaults.example.com"
			maxAge := "1728000"

			e2e.RunTestCase(t, &e2e.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					e2e.BasicIngress().
						WithName("cors-defaults").
						WithHost(host).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						Build(),
				},
				Verifiers: map[string][]e2e.Verifier{
					"cors-defaults": {
						&e2e.HttpGetVerifier{
							Host:   host,
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
							HeaderMatches: []e2e.HeaderMatch{
								{
									Name:     "Access-Control-Allow-Origin",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`^\*$|^` + regexp.QuoteMeta(origin) + `$`)},
								},
								{
									Name: "Access-Control-Allow-Methods",
									Patterns: []*regexp.Regexp{
										regexp.MustCompile(`(?i).*GET.*`),
										regexp.MustCompile(`(?i).*PUT.*`),
										regexp.MustCompile(`(?i).*POST.*`),
										regexp.MustCompile(`(?i).*DELETE.*`),
										regexp.MustCompile(`(?i).*PATCH.*`),
										regexp.MustCompile(`(?i).*OPTIONS.*`),
									},
								},
								{
									Name: "Access-Control-Allow-Headers",
									Patterns: []*regexp.Regexp{
										regexp.MustCompile(`(?i).*DNT.*`),
										regexp.MustCompile(`(?i).*Keep-Alive.*`),
										regexp.MustCompile(`(?i).*User-Agent.*`),
										regexp.MustCompile(`(?i).*X-Requested-With.*`),
										regexp.MustCompile(`(?i).*If-Modified-Since.*`),
										regexp.MustCompile(`(?i).*Cache-Control.*`),
										regexp.MustCompile(`(?i).*Content-Type.*`),
										regexp.MustCompile(`(?i).*Range.*`),
										regexp.MustCompile(`(?i).*Authorization.*`),
									},
								},
								{
									Name:     "Access-Control-Allow-Credentials",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^true$`)},
								},
								{
									Name:     "Access-Control-Max-Age",
									Patterns: []*regexp.Regexp{regexp.MustCompile("^" + maxAge + "$")},
								},
							},
						},
						&e2e.HttpGetVerifier{
							Host:   host,
							Path:   "/",
							Method: http.MethodGet,
							RequestHeaders: map[string]string{
								"Origin": origin,
							},
							HeaderMatches: []e2e.HeaderMatch{
								{
									Name:     "Access-Control-Allow-Origin",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`^\*$|^` + regexp.QuoteMeta(origin) + `$`)},
								},
								{
									Name:     "Access-Control-Allow-Credentials",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^true$`)},
								},
							},
						},
					},
				},
			})
		})
		t.Run("cors denied origin", func(t *testing.T) {
			suffix, err := e2e.RandString(6)
			require.NoError(t, err)
			host := fmt.Sprintf("cors-denied-%s.com", suffix)
			allowedOrigin := "https://cors-allowed.example.com"
			deniedOrigin := "https://cors-denied.example.com"

			e2e.RunTestCase(t, &e2e.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					e2e.BasicIngress().
						WithName("cors-denied").
						WithHost(host).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", allowedOrigin).
						Build(),
				},
				Verifiers: map[string][]e2e.Verifier{
					"cors-denied": {
						&e2e.HttpGetVerifier{
							Host:   host,
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
						&e2e.HttpGetVerifier{
							Host:   host,
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
			suffix, err := e2e.RandString(6)
			require.NoError(t, err)
			host := fmt.Sprintf("cors-denied-method-%s.com", suffix)
			origin := "https://cors-method.example.com"
			allowedMethods := "GET, POST"
			allowedHeaders := "X-Requested-With"
			deniedMethod := "DELETE"
			deniedHeader := "X-Not-Allowed"

			e2e.RunTestCase(t, &e2e.TestCase{
				GatewayImplementation:  istio.ProviderName,
				AllowExperimentalGWAPI: true,
				Providers:              []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					e2e.BasicIngress().
						WithName("cors-denied-method").
						WithHost(host).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/enable-cors", "true").
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-origin", origin).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-methods", allowedMethods).
						WithAnnotation("nginx.ingress.kubernetes.io/cors-allow-headers", allowedHeaders).
						Build(),
				},
				Verifiers: map[string][]e2e.Verifier{
					"cors-denied-method": {
						&e2e.HttpGetVerifier{
							Host:   host,
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
							HeaderMatches: []e2e.HeaderMatch{
								{
									Name:     "Access-Control-Allow-Origin",
									Patterns: []*regexp.Regexp{regexp.MustCompile("^" + regexp.QuoteMeta(origin) + "$")},
								},
								{
									Name: "Access-Control-Allow-Methods",
									Patterns: []*regexp.Regexp{
										regexp.MustCompile(`(?i).*GET.*`),
										regexp.MustCompile(`(?i).*POST.*`),
									},
								},
								{
									Name: "Access-Control-Allow-Headers",
									Patterns: []*regexp.Regexp{
										regexp.MustCompile(`(?i).*` + regexp.QuoteMeta(allowedHeaders) + `.*`),
									},
								},
							},
							HeaderExcludes: []e2e.HeaderExclude{
								{
									Name:     "Access-Control-Allow-Methods",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)` + deniedMethod)},
								},
								{
									Name:     "Access-Control-Allow-Headers",
									Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)` + regexp.QuoteMeta(deniedHeader))},
								},
							},
						},
					},
				},
			})
		})
	})
}
