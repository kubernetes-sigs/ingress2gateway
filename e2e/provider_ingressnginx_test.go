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

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// ingress-nginx provider features: One test per feature, all using Istio + standard emitter.

func TestIngressNGINXCanary(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("base canary", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err)
			host := fmt.Sprintf("canary-%s.com", suffix)
			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("foo1").
						WithHost(host).
						WithIngressClass(ingressnginx.NginxIngressClass).
						Build(),
					framework.BasicIngress().
						WithName("foo2").
						WithHost(host).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/canary", "true").
						WithAnnotation("nginx.ingress.kubernetes.io/canary-weight", "20").
						WithBackend(framework.DummyAppName2).
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"foo1": {
						&framework.CanaryVerifier{
							Verifier: &framework.HTTPRequestVerifier{
								Host:      host,
								Path:      "/hostname",
								BodyRegex: regexp.MustCompile("^dummy-app2"),
							},
							Runs:         200,
							MinSuccesses: 0.1,
							MaxSuccesses: 0.3,
						},
					},
				},
			})
		})
	})
}

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

func TestIngressNGINXPathRewrite(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("basic conversion", func(t *testing.T) {
			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("foo1").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithPath("/abc").
						WithAnnotation("nginx.ingress.kubernetes.io/rewrite-target", "/header").
						WithAnnotation("nginx.ingress.kubernetes.io/x-forwarded-prefix", "/abc").
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"foo1": {
						&framework.HTTPRequestVerifier{Path: "/abc", BodyRegex: regexp.MustCompile(`"X-Forwarded-Prefix":\["/abc"\]`)},
					},
				},
			})
		})
	})
}

func TestIngressNGINXTLS(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("tls ingress and gateway", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err, "creating host suffix")
			host := "tls-" + suffix + ".example.com"
			tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-cert-"+suffix, "", host, []string{host})
			if err != nil {
				t.Fatalf("creating TLS secret: %v", err)
			}
			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Secrets: []*corev1.Secret{tlsSecret.Secret},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("foo").
						WithHost(host).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithTLSSecret(tlsSecret.Secret.Name, host).
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"foo": {
						&framework.HTTPRequestVerifier{
							Host:      host,
							Path:      "/",
							UseTLS:    true,
							CACertPEM: tlsSecret.CACert,
						},
						&framework.HTTPRequestVerifier{
							Host:         host,
							Path:         "/",
							AllowedCodes: []int{308},
							UseTLS:       false,
							HeaderMatches: []framework.HeaderMatch{
								{
									Name: "Location",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile("^https://" + host + "/?$")},
									},
								},
							},
						},
					},
				},
			})
		})
		t.Run("ssl-redirect annotation", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err, "creating host suffix")
			redirectHost := "tls-redirect-" + suffix + ".example.com"
			noRedirectHost := "tls-noredirect-" + suffix + ".example.com"
			redirectSecret, err := framework.GenerateSelfSignedTLSSecret("tls-redirect-"+suffix, "", redirectHost, []string{redirectHost})
			if err != nil {
				t.Fatalf("creating redirect TLS secret: %v", err)
			}
			noRedirectSecret, err := framework.GenerateSelfSignedTLSSecret("tls-noredirect-"+suffix, "", noRedirectHost, []string{noRedirectHost})
			if err != nil {
				t.Fatalf("creating no-redirect TLS secret: %v", err)
			}
			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Secrets: []*corev1.Secret{redirectSecret.Secret, noRedirectSecret.Secret},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("redirect").
						WithHost(redirectHost).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithTLSSecret(redirectSecret.Secret.Name, redirectHost).
						Build(),
					framework.BasicIngress().
						WithName("no-redirect").
						WithHost(noRedirectHost).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
						WithTLSSecret(noRedirectSecret.Secret.Name, noRedirectHost).
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"redirect": {
						&framework.HTTPRequestVerifier{
							Host:      redirectHost,
							Path:      "/",
							UseTLS:    true,
							CACertPEM: redirectSecret.CACert,
						},
						&framework.HTTPRequestVerifier{
							Host:         redirectHost,
							Path:         "/",
							AllowedCodes: []int{308},
							UseTLS:       false,
							HeaderMatches: []framework.HeaderMatch{
								{
									Name: "Location",
									Patterns: []*framework.MaybeNegativePattern{
										{Pattern: regexp.MustCompile("^https://" + redirectHost + "/?$")},
									},
								},
							},
						},
					},
					"no-redirect": {
						&framework.HTTPRequestVerifier{
							Host:      noRedirectHost,
							Path:      "/",
							UseTLS:    true,
							CACertPEM: noRedirectSecret.CACert,
						},
						&framework.HTTPRequestVerifier{
							Host:   noRedirectHost,
							Path:   "/",
							UseTLS: false,
						},
					},
				},
			})
		})
	})
}

const slowShellPath = "/shell?cmd=sleep%204%3B%20echo%20done"
const verySlowShellPath = "/shell?cmd=sleep%2015%3B%20echo%20done"

func TestIngressNGINXTimeouts(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("slow response allowed", func(t *testing.T) {
			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("slow-allowed").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithPath("/shell").
						WithAnnotation(ingressnginx.ProxyConnectTimeoutAnnotation, "5").
						WithAnnotation(ingressnginx.ProxyReadTimeoutAnnotation, "5").
						WithAnnotation(ingressnginx.ProxySendTimeoutAnnotation, "5").
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"slow-allowed": {
						&framework.HTTPRequestVerifier{
							Path:      slowShellPath,
							BodyRegex: regexp.MustCompile("done"),
						},
					},
				},
			})
		})
		t.Run("short timeout", func(t *testing.T) {
			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("short-timeout").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithPath("/shell").
						WithAnnotation(ingressnginx.ProxyConnectTimeoutAnnotation, "1").
						WithAnnotation(ingressnginx.ProxyReadTimeoutAnnotation, "1").
						WithAnnotation(ingressnginx.ProxySendTimeoutAnnotation, "1").
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"short-timeout": {
						&framework.HTTPRequestVerifier{
							Path:         verySlowShellPath,
							AllowedCodes: []int{http.StatusGatewayTimeout, http.StatusInternalServerError},
						},
					},
				},
			})
		})
	})
}
