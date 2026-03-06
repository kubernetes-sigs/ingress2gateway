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
	t.Run("canary by header at path", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := fmt.Sprintf("canary-header-path-%s.com", suffix)
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
					WithName("main").
					WithHost(host).
					WithPath("/hostname").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithBackend(framework.DummyAppName1).
					Build(),
				framework.BasicIngress().
					WithName("canary-header").
					WithHost(host).
					WithPath("/hostname").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/canary", "true").
					WithAnnotation("nginx.ingress.kubernetes.io/canary-by-header", "X-Canary").
					WithBackend(framework.DummyAppName2).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"main": {
					// With the canary header set to "always", all requests at the
					// canary path should go to the canary backend.
					&framework.HTTPRequestVerifier{
						Host: host,
						Path: "/hostname",
						RequestHeaders: map[string]string{
							"X-Canary": "always",
						},
						BodyRegex: regexp.MustCompile("^dummy-app2"),
					},
					// Without the header, requests should go to the main backend.
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/hostname",
						BodyRegex: regexp.MustCompile("^dummy-app1"),
					},
				},
			},
		})
	})
	t.Run("canary weight and header combined", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := fmt.Sprintf("canary-combined-%s.com", suffix)
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
					WithName("prod").
					WithHost(host).
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithBackend(framework.DummyAppName1).
					Build(),
				framework.BasicIngress().
					WithName("canary-combined").
					WithHost(host).
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/canary", "true").
					WithAnnotation("nginx.ingress.kubernetes.io/canary-weight", "20").
					WithAnnotation("nginx.ingress.kubernetes.io/canary-by-header", "X-Canary").
					WithBackend(framework.DummyAppName2).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"prod": {
					// With the canary header set to "always", 100% of requests
					// should go to the canary backend regardless of weight.
					&framework.HTTPRequestVerifier{
						Host: host,
						Path: "/hostname",
						RequestHeaders: map[string]string{
							"X-Canary": "always",
						},
						BodyRegex: regexp.MustCompile("^dummy-app2"),
					},
					// With the canary header set to "never", 0% of requests
					// should go to the canary backend regardless of weight.
					&framework.HTTPRequestVerifier{
						Host: host,
						Path: "/hostname",
						RequestHeaders: map[string]string{
							"X-Canary": "never",
						},
						BodyRegex: regexp.MustCompile("^dummy-app1"),
					},
					// Without any header, the canary-weight (20%) applies.
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
}

func TestIngressNGINXCORS(t *testing.T) {
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
}

func TestIngressNGINXPathRewrite(t *testing.T) {
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
}

func TestIngressNGINXTLS(t *testing.T) {
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
	t.Run("conflicting ssl-redirect disabled wins", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-conflict-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-conflict-"+suffix, "", host, []string{host})
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
					WithName("redirect-enabled").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "true").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				framework.BasicIngress().
					WithName("redirect-disabled").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"redirect-enabled": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /a should redirect because its source ingress has ssl-redirect=true.
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"redirect-disabled": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b should NOT redirect because its source ingress has ssl-redirect=false.
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/b",
						UseTLS: false,
					},
				},
			},
		})
	})
	t.Run("three rules mixed ssl-redirect", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-3mix-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-3mix-"+suffix, "", host, []string{host})
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
					WithName("rule-a-enabled").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				framework.BasicIngress().
					WithName("rule-b-disabled").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				framework.BasicIngress().
					WithName("rule-c-enabled").
					WithHost(host).
					WithPath("/c").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"rule-a-enabled": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /a should redirect because its source ingress has ssl-redirect=true (default).
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"rule-b-disabled": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b should NOT redirect because its source ingress has ssl-redirect=false.
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/b",
						UseTLS: false,
					},
				},
				"rule-c-enabled": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/c",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /c should redirect because its source ingress has ssl-redirect=true (default).
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/c",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
			},
		})
	})
	t.Run("cross-ingress TLS redirect", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-cross-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-cross-"+suffix, "", host, []string{host})
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
				// Ingress A: has TLS, default ssl-redirect=true
				framework.BasicIngress().
					WithName("with-tls").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				// Ingress B: no TLS section, but shares host with TLS ingress
				framework.BasicIngress().
					WithName("no-tls").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"with-tls": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /a should redirect (TLS ingress, default enabled)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"no-tls": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b should also redirect — hostname has TLS from the other ingress
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/b",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
			},
		})
	})
	t.Run("cross-ingress TLS with opt-out", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-crossopt-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-crossopt-"+suffix, "", host, []string{host})
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
				// Ingress A: has TLS, default ssl-redirect=true
				framework.BasicIngress().
					WithName("with-tls").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				// Ingress B: no TLS, explicit ssl-redirect=false
				framework.BasicIngress().
					WithName("no-tls-optout").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"with-tls": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /a should redirect (TLS ingress, default enabled)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"no-tls-optout": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b should NOT redirect (explicit ssl-redirect=false)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/b",
						UseTLS: false,
					},
				},
			},
		})
	})
	t.Run("three-way mixed cross-ingress TLS", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-3way-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-3way-"+suffix, "", host, []string{host})
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
				// Ingress A: has TLS, default ssl-redirect=true
				framework.BasicIngress().
					WithName("tls-default").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				// Ingress B: no TLS, explicit ssl-redirect=false
				framework.BasicIngress().
					WithName("no-tls-optout").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					Build(),
				// Ingress C: no TLS, default ssl-redirect (inherits from hostname)
				framework.BasicIngress().
					WithName("no-tls-default").
					WithHost(host).
					WithPath("/c").
					WithIngressClass(ingressnginx.NginxIngressClass).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"tls-default": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"no-tls-optout": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b should NOT redirect (explicit opt-out)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/b",
						UseTLS: false,
					},
				},
				"no-tls-default": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/c",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /c should redirect (hostname has TLS, default ssl-redirect=true)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/c",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
			},
		})
	})
	t.Run("all rules disabled with TLS", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-alldis-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-alldis-"+suffix, "", host, []string{host})
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
					WithName("disabled-a").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				framework.BasicIngress().
					WithName("disabled-b").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"disabled-a": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /a should NOT redirect (explicit ssl-redirect=false)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/a",
						UseTLS: false,
					},
				},
				"disabled-b": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b should NOT redirect (explicit ssl-redirect=false)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/b",
						UseTLS: false,
					},
				},
			},
		})
	})
	t.Run("TLS ingress opts out non-TLS inherits", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-inherit-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-inherit-"+suffix, "", host, []string{host})
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
				// TLS ingress explicitly disables ssl-redirect
				framework.BasicIngress().
					WithName("tls-optout").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				// Non-TLS ingress with default — inherits redirect from hostname TLS
				framework.BasicIngress().
					WithName("no-tls-inherits").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"tls-optout": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /a should NOT redirect (explicit ssl-redirect=false)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/a",
						UseTLS: false,
					},
				},
				"no-tls-inherits": {
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b SHOULD redirect — hostname has TLS, default ssl-redirect=true
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/b",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
			},
		})
	})
	t.Run("no redirect same routes on both ports", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "tls-bothports-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("tls-bothports-"+suffix, "", host, []string{host})
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
				// Ingress with TLS + ssl-redirect=false
				framework.BasicIngress().
					WithName("tls-noredirect").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
				// Ingress without TLS + ssl-redirect=false, same host
				framework.BasicIngress().
					WithName("plain-noredirect").
					WithHost(host).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"tls-noredirect": {
					// /a reachable on HTTPS
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /a also reachable on HTTP (no redirect)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/a",
						UseTLS: false,
					},
				},
				"plain-noredirect": {
					// /b reachable on HTTPS
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// /b also reachable on HTTP (no redirect)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/b",
						UseTLS: false,
					},
				},
			},
		})
	})
	t.Run("TLS scoped to one host only", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		hostA := "scoped-a-" + suffix + ".example.com"
		hostB := "scoped-b-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("scoped-tls-"+suffix, "", hostA, []string{hostA})
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
					WithName("scoped-tls").
					WithHost(hostA).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, hostA).
					Build(),
				framework.BasicIngress().
					WithName("no-tls-host").
					WithHost(hostB).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"scoped-tls": {
					&framework.HTTPRequestVerifier{
						Host:      hostA,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// host-a /a should redirect on HTTP
					&framework.HTTPRequestVerifier{
						Host:         hostA,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"no-tls-host": {
					// host-b /b serves normally on HTTP (no TLS, no redirect)
					&framework.HTTPRequestVerifier{
						Host:   hostB,
						Path:   "/b",
						UseTLS: false,
					},
				},
			},
		})
	})
	t.Run("two ingresses different hosts only one TLS", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		hostA := "diffhost-a-" + suffix + ".example.com"
		hostB := "diffhost-b-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("diffhost-tls-"+suffix, "", hostA, []string{hostA})
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
					WithName("diffhost-tls").
					WithHost(hostA).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, hostA).
					Build(),
				framework.BasicIngress().
					WithName("diffhost-notls").
					WithHost(hostB).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"diffhost-tls": {
					&framework.HTTPRequestVerifier{
						Host:      hostA,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
					// host-a /a should redirect on HTTP
					&framework.HTTPRequestVerifier{
						Host:         hostA,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"diffhost-notls": {
					// host-b /b serves normally on HTTP (no TLS, no redirect)
					&framework.HTTPRequestVerifier{
						Host:   hostB,
						Path:   "/b",
						UseTLS: false,
					},
				},
			},
		})
	})
	t.Run("ssl-redirect true without TLS", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "notls-redir-" + suffix + ".example.com"
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
					WithName("notls-redir").
					WithHost(host).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "true").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"notls-redir": {
					// HTTP serves normally (no redirect — no cert for the host)
					&framework.HTTPRequestVerifier{
						Host:   host,
						Path:   "/a",
						UseTLS: false,
					},
				},
			},
		})
	})
	t.Run("TLS with explicit host list subset of rules", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		hostA := "subset-a-" + suffix + ".example.com"
		hostB := "subset-b-" + suffix + ".example.com"
		tlsSecretA, err := framework.GenerateSelfSignedTLSSecret("subset-a-"+suffix, "", hostA, []string{hostA})
		if err != nil {
			t.Fatalf("creating TLS secret A: %v", err)
		}
		tlsSecretB, err := framework.GenerateSelfSignedTLSSecret("subset-b-"+suffix, "", hostB, []string{hostB})
		if err != nil {
			t.Fatalf("creating TLS secret B: %v", err)
		}
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags: map[string]map[string]string{
				ingressnginx.Name: {
					ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
				},
			},
			Secrets: []*corev1.Secret{tlsSecretA.Secret, tlsSecretB.Secret},
			Ingresses: []*networkingv1.Ingress{
				// Ingress A: host-a with TLS for host-a only
				framework.BasicIngress().
					WithName("subset-a").
					WithHost(hostA).
					WithPath("/a").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecretA.Secret.Name, hostA).
					Build(),
				// Ingress B: host-b with TLS for host-b only
				framework.BasicIngress().
					WithName("subset-b").
					WithHost(hostB).
					WithPath("/b").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithTLSSecret(tlsSecretB.Secret.Name, hostB).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"subset-a": {
					&framework.HTTPRequestVerifier{
						Host:      hostA,
						Path:      "/a",
						UseTLS:    true,
						CACertPEM: tlsSecretA.CACert,
					},
					// host-a /a should redirect independently
					&framework.HTTPRequestVerifier{
						Host:         hostA,
						Path:         "/a",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
				"subset-b": {
					&framework.HTTPRequestVerifier{
						Host:      hostB,
						Path:      "/b",
						UseTLS:    true,
						CACertPEM: tlsSecretB.CACert,
					},
					// host-b /b should redirect independently
					&framework.HTTPRequestVerifier{
						Host:         hostB,
						Path:         "/b",
						AllowedCodes: []int{308},
						UseTLS:       false,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^https://")},
								},
							},
						},
					},
				},
			},
		})
	})
}

const slowShellPath = "/shell?cmd=sleep%204%3B%20echo%20done"
const verySlowShellPath = "/shell?cmd=sleep%2015%3B%20echo%20done"

func TestIngressNGINXTimeouts(t *testing.T) {
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
}

func TestIngressNGINXRedirect(t *testing.T) {
	t.Parallel()
	t.Run("permanent redirect", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		redirectURL := fmt.Sprintf("https://new-site-%s.example.com/new-path/", suffix)

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
					WithName("permanent-redirect").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", redirectURL).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"permanent-redirect": {
					&framework.HTTPRequestVerifier{
						Path: "/",
						AllowedCodes: []int{
							http.StatusMovedPermanently, // 301
						},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})

	t.Run("temporal redirect", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		redirectURL := fmt.Sprintf("https://temp-site-%s.example.com/temp-path/", suffix)

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
					WithName("temporal-redirect").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", redirectURL).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"temporal-redirect": {
					&framework.HTTPRequestVerifier{
						Path: "/",
						AllowedCodes: []int{
							http.StatusFound, // 302
						},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})

	t.Run("permanent redirect with supported custom code", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		redirectURL := fmt.Sprintf("https://custom-code-%s.example.com/path/", suffix)

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
					WithName("permanent-redirect-301").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", redirectURL).
					WithAnnotation("nginx.ingress.kubernetes.io/permanent-redirect-code", "301").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"permanent-redirect-301": {
					&framework.HTTPRequestVerifier{
						Path: "/",
						AllowedCodes: []int{
							http.StatusMovedPermanently, // 301
						},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})

	t.Run("temporal redirect with supported custom code", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		redirectURL := fmt.Sprintf("https://custom-temp-%s.example.com/path/", suffix)

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
					WithName("temporal-redirect-302").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", redirectURL).
					WithAnnotation("nginx.ingress.kubernetes.io/temporal-redirect-code", "302").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"temporal-redirect-302": {
					&framework.HTTPRequestVerifier{
						Path: "/",
						AllowedCodes: []int{
							http.StatusFound, // 302
						},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})

	t.Run("redirect with scheme and hostname only", func(t *testing.T) {
		redirectURL := "https://another-domain.example.com/"

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
					WithName("redirect-hostname-only").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", redirectURL).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"redirect-hostname-only": {
					&framework.HTTPRequestVerifier{
						Path: "/some/path",
						AllowedCodes: []int{
							http.StatusMovedPermanently, // 301
						},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})

	t.Run("redirect with port", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		redirectURL := fmt.Sprintf("https://custom-port-%s.example.com:8443/secure/", suffix)

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
					WithName("redirect-with-port").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", redirectURL).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"redirect-with-port": {
					&framework.HTTPRequestVerifier{
						Path: "/",
						AllowedCodes: []int{
							http.StatusFound, // 302
						},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})

	t.Run("both redirect annotations - temporal takes priority", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		permanentURL := fmt.Sprintf("https://permanent-%s.example.com/path/", suffix)
		temporalURL := fmt.Sprintf("https://temporal-%s.example.com/path/", suffix)

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
					WithName("both-redirects").
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", permanentURL).
					WithAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", temporalURL).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"both-redirects": {
					&framework.HTTPRequestVerifier{
						Path: "/",
						AllowedCodes: []int{
							http.StatusFound, // 302 - temporal takes priority
						},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(temporalURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})
	t.Run("permanent redirect with TLS and ssl-redirect false", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := "redir-nossl-" + suffix + ".example.com"
		redirectURL := fmt.Sprintf("https://new-site-%s.example.com/path/", suffix)
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("redir-nossl-"+suffix, "", host, []string{host})
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
					WithName("permanent-redirect-nossl").
					WithHost(host).
					WithIngressClass(ingressnginx.NginxIngressClass).
					WithAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", redirectURL).
					WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
					WithTLSSecret(tlsSecret.Secret.Name, host).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"permanent-redirect-nossl": {
					// HTTPS: should get 301 redirect to the URL
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/",
						UseTLS:       true,
						CACertPEM:    tlsSecret.CACert,
						AllowedCodes: []int{http.StatusMovedPermanently},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
					// HTTP: should get 301 redirect to the URL directly (no ssl-redirect)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/",
						UseTLS:       false,
						AllowedCodes: []int{http.StatusMovedPermanently},
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Location",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
								},
							},
						},
					},
				},
			},
		})
	})
}

func TestIngressNGINXRegex(t *testing.T) {
	t.Parallel()
	t.Run("host-level matching", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		regexHost := fmt.Sprintf("regex-host-%s.example.com", suffix)
		implementationSpecific := networkingv1.PathTypeImplementationSpecific
		exactPathType := networkingv1.PathTypeExact

		plain := framework.BasicIngress().
			WithName("plain").
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithHost(regexHost).
			WithPath("/hoSTn"). // Check for case-insensitivity of regex matching
			Build()
		// Exact becomes regex which are prefix
		plain.Spec.Rules[0].HTTP.Paths[0].PathType = &exactPathType

		regex := framework.BasicIngress().
			WithName("regex").
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithHost(regexHost).
			WithPath("/cliEnt.+"). // Check for case-insensitivity of regex matching
			WithAnnotation(ingressnginx.UseRegexAnnotation, "true").
			Build()
		regex.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags: map[string]map[string]string{
				ingressnginx.Name: {
					ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
				},
			},
			Ingresses: []*networkingv1.Ingress{plain, regex},
			Verifiers: map[string][]framework.Verifier{
				"plain": {
					&framework.HTTPRequestVerifier{
						Host: regexHost,
						Path: "/hostname",
					},
				},
				"regex": {
					&framework.HTTPRequestVerifier{
						Host: regexHost,
						Path: "/clientip",
					},
				},
			},
		})
	})
	t.Run("rewrite-target implies host-level matching", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		regexHost := fmt.Sprintf("rewrite-regex-host-%s.example.com", suffix)
		implementationSpecific := networkingv1.PathTypeImplementationSpecific
		exactPathType := networkingv1.PathTypeExact

		plain := framework.BasicIngress().
			WithName("plain").
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithHost(regexHost).
			WithPath("/hostn").
			Build()
		plain.Spec.Rules[0].HTTP.Paths[0].PathType = &exactPathType

		rewriteRegex := framework.BasicIngress().
			WithName("rewrite-regex").
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithHost(regexHost).
			WithPath("/client.+").
			WithAnnotation(ingressnginx.RewriteTargetAnnotation, "/").
			Build()
		rewriteRegex.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags: map[string]map[string]string{
				ingressnginx.Name: {
					ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
				},
			},
			Ingresses: []*networkingv1.Ingress{plain, rewriteRegex},
			Verifiers: map[string][]framework.Verifier{
				"plain": {
					&framework.HTTPRequestVerifier{
						Host: regexHost,
						Path: "/hostname",
					},
				},
				"rewrite-regex": {
					&framework.HTTPRequestVerifier{
						Host: regexHost,
						Path: "/clientip",
					},
				},
			},
		})
	})
	t.Run("regex ending with dollar matches only exact path", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := fmt.Sprintf("regex-dollar-%s.example.com", suffix)
		implementationSpecific := networkingv1.PathTypeImplementationSpecific

		ing := framework.BasicIngress().
			WithName("dollar").
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithHost(host).
			WithPath("/$").
			WithAnnotation(ingressnginx.UseRegexAnnotation, "true").
			Build()
		ing.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags: map[string]map[string]string{
				ingressnginx.Name: {
					ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
				},
			},
			Ingresses: []*networkingv1.Ingress{ing},
			Verifiers: map[string][]framework.Verifier{
				"dollar": {
					&framework.HTTPRequestVerifier{
						Host: host,
						Path: "/",
					},
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						AllowedCodes: []int{404},
					},
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname/",
						AllowedCodes: []int{404},
					},
				},
			},
		})
	})
}

func TestIngressNGINXTrailingSlashRedirect(t *testing.T) {
	t.Parallel()

	providerFlags := map[string]map[string]string{
		ingressnginx.Name: {
			ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
		},
	}
	prefixPathType := networkingv1.PathTypePrefix
	exactPathType := networkingv1.PathTypeExact

	t.Run("prefix path with trailing slash", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := "trailing-prefix-" + suffix + ".example.com"

		ing := framework.BasicIngress().
			WithName("prefix-slash").
			WithHost(host).
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithPath("/hostname/").
			Build()
		ing.Spec.Rules[0].HTTP.Paths[0].PathType = &prefixPathType

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags:         providerFlags,
			Ingresses:             []*networkingv1.Ingress{ing},
			Verifiers: map[string][]framework.Verifier{
				"prefix-slash": {
					// /hostname should get a 301 redirect to /hostname/
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						AllowedCodes: []int{301},
						HeaderMatches: []framework.HeaderMatch{{
							Name: "Location",
							Patterns: []*framework.MaybeNegativePattern{
								{Pattern: regexp.MustCompile(`^http://`)},
								{Pattern: regexp.MustCompile(`/hostname/$`)},
								{Pattern: regexp.MustCompile(`^https://`), Negate: true},
							},
						}},
					},
					// /hostname/ should return 200
					&framework.HTTPRequestVerifier{
						Host: host,
						Path: "/hostname/",
					},
				},
			},
		})
	})

	t.Run("exact path with trailing slash", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := "trailing-exact-" + suffix + ".example.com"

		ing := framework.BasicIngress().
			WithName("exact-slash").
			WithHost(host).
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithPath("/hostname/").
			Build()
		ing.Spec.Rules[0].HTTP.Paths[0].PathType = &exactPathType

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags:         providerFlags,
			Ingresses:             []*networkingv1.Ingress{ing},
			Verifiers: map[string][]framework.Verifier{
				"exact-slash": {
					// /hostname should get a 301 redirect to /hostname/
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						AllowedCodes: []int{301},
						HeaderMatches: []framework.HeaderMatch{{
							Name: "Location",
							Patterns: []*framework.MaybeNegativePattern{
								{Pattern: regexp.MustCompile(`^http://`)},
								{Pattern: regexp.MustCompile(`/hostname/$`)},
								{Pattern: regexp.MustCompile(`^https://`), Negate: true},
							},
						}},
					},
					// /hostname/ should return 200
					&framework.HTTPRequestVerifier{
						Host: host,
						Path: "/hostname/",
					},
				},
			},
		})
	})

	t.Run("no redirect for path without trailing slash", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := "trailing-none-" + suffix + ".example.com"

		ing := framework.BasicIngress().
			WithName("no-slash").
			WithHost(host).
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithPath("/hostname").
			Build()
		ing.Spec.Rules[0].HTTP.Paths[0].PathType = &prefixPathType

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags:         providerFlags,
			Ingresses:             []*networkingv1.Ingress{ing},
			Verifiers: map[string][]framework.Verifier{
				"no-slash": {
					// /hostname should return 200 with no redirect (path has no trailing slash)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						HeaderAbsent: []string{"Location"},
					},
				},
			},
		})
	})

	t.Run("trailing slash with TLS ssl-redirect", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := "trailing-tls-" + suffix + ".example.com"

		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("trailing-tls-cert-"+suffix, "", host, []string{host})
		require.NoError(t, err, "creating TLS secret")

		ing := framework.BasicIngress().
			WithName("tls-slash").
			WithHost(host).
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithPath("/hostname/").
			WithTLSSecret(tlsSecret.Secret.Name, host).
			Build()
		ing.Spec.Rules[0].HTTP.Paths[0].PathType = &prefixPathType

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags:         providerFlags,
			Secrets:               []*corev1.Secret{tlsSecret.Secret},
			Ingresses:             []*networkingv1.Ingress{ing},
			Verifiers: map[string][]framework.Verifier{
				"tls-slash": {
					// HTTP /hostname → some redirect (301 or 308, depending on ordering)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						AllowedCodes: []int{301, 308},
					},
					// HTTP /hostname/ → SSL redirect (308 to https)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname/",
						AllowedCodes: []int{308},
						HeaderMatches: []framework.HeaderMatch{{
							Name: "Location",
							Patterns: []*framework.MaybeNegativePattern{
								{Pattern: regexp.MustCompile(`^https://`)},
							},
						}},
					},
					// HTTPS /hostname → trailing slash redirect (301 to /hostname/)
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						UseTLS:       true,
						CACertPEM:    tlsSecret.CACert,
						AllowedCodes: []int{301},
						HeaderMatches: []framework.HeaderMatch{{
							Name: "Location",
							Patterns: []*framework.MaybeNegativePattern{
								{Pattern: regexp.MustCompile(`/hostname/$`)},
							},
						}},
					},
					// HTTPS /hostname/ → 200
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/hostname/",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
					},
				},
			},
		})
	})

	t.Run("no trailing slash redirect with TLS when path has no slash", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := "trailing-tls-noslash-" + suffix + ".example.com"

		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("trailing-tls-noslash-"+suffix, "", host, []string{host})
		require.NoError(t, err, "creating TLS secret")

		ing := framework.BasicIngress().
			WithName("tls-noslash").
			WithHost(host).
			WithIngressClass(ingressnginx.NginxIngressClass).
			WithPath("/hostname").
			WithTLSSecret(tlsSecret.Secret.Name, host).
			Build()
		ing.Spec.Rules[0].HTTP.Paths[0].PathType = &prefixPathType

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: istio.ProviderName,
			Providers:             []string{ingressnginx.Name},
			ProviderFlags:         providerFlags,
			Secrets:               []*corev1.Secret{tlsSecret.Secret},
			Ingresses:             []*networkingv1.Ingress{ing},
			Verifiers: map[string][]framework.Verifier{
				"tls-noslash": {
					// HTTP /hostname → straight SSL redirect (308 to https), no trailing slash redirect
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						AllowedCodes: []int{308},
						HeaderMatches: []framework.HeaderMatch{{
							Name: "Location",
							Patterns: []*framework.MaybeNegativePattern{
								{Pattern: regexp.MustCompile(`^https://`)},
							},
						}},
					},
					// HTTPS /hostname → 200 directly
					&framework.HTTPRequestVerifier{
						Host:         host,
						Path:         "/hostname",
						UseTLS:       true,
						CACertPEM:    tlsSecret.CACert,
						HeaderAbsent: []string{"Location"},
					},
				},
			},
		})
	})
}
