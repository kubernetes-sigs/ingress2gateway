/*
Copyright The Kubernetes Authors.

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
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// just keeping this here so it works for now
const (
	backendTLSAppName   = "tls-backend"
	backendServerSecret = "tls-backend-server-cert"
	backendCASecretName = "tls-backend-ca"
)

// ingress-nginx provider features: One test per feature, all using Istio + standard emitter.

// backendTLSSetup creates TLS secrets, a CA ConfigMap, and deploys an HTTPS backend app.
// It is called between setupTestEnv and env.Run() so that TLS secrets exist before the
// dummy TLS app pod tries to mount them.
func backendTLSSetup(env *framework.TestEnv) {
	env.T.Helper()

	namespace := env.Namespace

	// The server hostname for the certificate must use the in-cluster DNS name.
	svcHost := fmt.Sprintf("%s.%s.svc.cluster.local", backendTLSAppName, namespace)

	secrets, err := framework.GenerateBackendTLSSecrets(backendServerSecret, backendCASecretName, namespace, svcHost)
	require.NoError(env.T, err, "generating backend TLS secrets")

	// Create the CA secret (referenced by proxy-ssl-secret annotation for NGINX ingress).
	secrets.CASecret.Namespace = namespace
	_, err = env.K8sClient.CoreV1().Secrets(namespace).Create(env.Ctx, secrets.CASecret, metav1.CreateOptions{})
	require.NoError(env.T, err, "creating CA secret")
	env.T.Cleanup(func() {
		if env.SkipCleanup {
			return
		}
		_ = env.K8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), secrets.CASecret.Name, metav1.DeleteOptions{})
	})

	// Create a ConfigMap with the same CA cert data. The converter outputs
	// caCertificateRefs with kind: ConfigMap (the standard kind supported by
	// Gateway API implementations like Istio). The ConfigMap name matches the
	// CA secret name so proxy-ssl-secret annotation value maps directly.
	caCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backendCASecretName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"ca.crt": string(secrets.CACertPEM),
		},
	}
	_, err = env.K8sClient.CoreV1().ConfigMaps(namespace).Create(env.Ctx, caCM, metav1.CreateOptions{})
	require.NoError(env.T, err, "creating CA configmap")
	env.T.Cleanup(func() {
		if env.SkipCleanup {
			return
		}
		_ = env.K8sClient.CoreV1().ConfigMaps(namespace).Delete(context.Background(), backendCASecretName, metav1.DeleteOptions{})
	})

	// Create the server TLS secret (mounted in the backend pod).
	secrets.ServerSecret.Namespace = namespace
	_, err = env.K8sClient.CoreV1().Secrets(namespace).Create(env.Ctx, secrets.ServerSecret, metav1.CreateOptions{})
	require.NoError(env.T, err, "creating server TLS secret")
	env.T.Cleanup(func() {
		if env.SkipCleanup {
			return
		}
		_ = env.K8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), secrets.ServerSecret.Name, metav1.DeleteOptions{})
	})

	// Deploy the TLS backend app.
	cleanupApp, err := framework.DeployDummyApp(env.Ctx, env.T, env.K8sClient, backendTLSAppName, namespace, env.SkipCleanup, true)
	require.NoError(env.T, err, "deploying TLS backend app")
	env.T.Cleanup(cleanupApp)
}

func TestIngressNGINXBackendTLS(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()

		// Test 1: Valid backend TLS configuration – all required annotations present.
		// The ingress2gateway tool should produce a BackendTLSPolicy for this ingress
		// and the gateway should be able to reach the HTTPS backend.
		t.Run("valid backend tls produces BackendTLSPolicy", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err, "creating host suffix")
			host := "backend-tls-valid-" + suffix + ".example.com"

			env := setupTestEnv(t, []string{ingressnginx.Name}, istio.ProviderName)
			backendTLSSetup(env)

			svcHost := fmt.Sprintf("%s.%s.svc.cluster.local", backendTLSAppName, env.Namespace)
			ing := framework.BasicIngress().
				WithName("backend-tls-valid").
				WithHost(host).
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithBackend(backendTLSAppName).
				WithBackendPort(443).
				WithAnnotation(ingressnginx.BackendProtocolAnnotation, "HTTPS").
				WithAnnotation(ingressnginx.ProxySSLVerifyAnnotation, "on").
				WithAnnotation(ingressnginx.ProxySSLSecretAnnotation, env.Namespace+"/"+backendCASecretName).
				WithAnnotation(ingressnginx.ProxySSLServerNameAnnotation, "on").
				WithAnnotation(ingressnginx.ProxySSLNameAnnotation, svcHost).
				Build()

			env.Run(&framework.TestCase{
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{ing},
				Verifiers: map[string][]framework.Verifier{
					"backend-tls-valid": {
						// The request should reach the HTTPS backend through the gateway
						// and return a 200 OK (agnhost netexec echoes back on /).
						&framework.HTTPRequestVerifier{
							Host: host,
							Path: "/",
						},
					},
				},
			})
		})

		// Test 2: Unsupported annotations produce warnings but don't block policy generation.
		// proxy-ssl-verify-depth and proxy-ssl-protocols should emit warnings but a valid
		// BackendTLSPolicy should still be produced when all required annotations are present.
		t.Run("unsupported annotations emit warnings but policy still generated", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err, "creating host suffix")
			host := "backend-tls-warn-" + suffix + ".example.com"

			env := setupTestEnv(t, []string{ingressnginx.Name}, istio.ProviderName)
			backendTLSSetup(env)

			svcHost := fmt.Sprintf("%s.%s.svc.cluster.local", backendTLSAppName, env.Namespace)
			ing := framework.BasicIngress().
				WithName("backend-tls-warn").
				WithHost(host).
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithBackend(backendTLSAppName).
				WithBackendPort(443).
				WithAnnotation(ingressnginx.BackendProtocolAnnotation, "HTTPS").
				WithAnnotation(ingressnginx.ProxySSLVerifyAnnotation, "on").
				WithAnnotation(ingressnginx.ProxySSLSecretAnnotation, env.Namespace+"/"+backendCASecretName).
				WithAnnotation(ingressnginx.ProxySSLServerNameAnnotation, "on").
				WithAnnotation(ingressnginx.ProxySSLNameAnnotation, svcHost).
				// These two annotations are unsupported in Gateway API
				// but should NOT prevent policy generation.
				WithAnnotation(ingressnginx.ProxySSLVerifyDepthAnnotation, "3").
				WithAnnotation(ingressnginx.ProxySSLProtocolsAnnotation, "TLSv1.2 TLSv1.3").
				Build()

			env.Run(&framework.TestCase{
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{ing},
				Verifiers: map[string][]framework.Verifier{
					"backend-tls-warn": {
						&framework.HTTPRequestVerifier{
							Host: host,
							Path: "/",
						},
					},
				},
			})
		})

		// Test 3: Valid config with body response verification – ensure the request
		// actually reaches the backend and we get a real response body.
		t.Run("valid backend tls with body verification", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err, "creating host suffix")
			host := "backend-tls-body-" + suffix + ".example.com"

			env := setupTestEnv(t, []string{ingressnginx.Name}, istio.ProviderName)
			backendTLSSetup(env)

			svcHost := fmt.Sprintf("%s.%s.svc.cluster.local", backendTLSAppName, env.Namespace)
			ing := framework.BasicIngress().
				WithName("backend-tls-body").
				WithHost(host).
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithBackend(backendTLSAppName).
				WithBackendPort(443).
				WithAnnotation(ingressnginx.BackendProtocolAnnotation, "HTTPS").
				WithAnnotation(ingressnginx.ProxySSLVerifyAnnotation, "on").
				WithAnnotation(ingressnginx.ProxySSLSecretAnnotation, env.Namespace+"/"+backendCASecretName).
				WithAnnotation(ingressnginx.ProxySSLServerNameAnnotation, "on").
				WithAnnotation(ingressnginx.ProxySSLNameAnnotation, svcHost).
				Build()

			env.Run(&framework.TestCase{
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{ing},
				Verifiers: map[string][]framework.Verifier{
					"backend-tls-body": {
						// agnhost netexec echoes back useful info on /hostname
						&framework.HTTPRequestVerifier{
							Host:      host,
							Path:      "/hostname",
							BodyRegex: regexp.MustCompile(`.+`),
						},
					},
				},
			})
		})
	})
}

func TestIngressNGINXCanary(t *testing.T) {
	t.Parallel()
	t.Run("base canary", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err)
		host := fmt.Sprintf("canary-%s.com", suffix)
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation:  implementation.IstioName,
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
			GatewayImplementation:  implementation.IstioName,
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
			GatewayImplementation:  implementation.IstioName,
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
			GatewayImplementation:  implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
}

const slowShellPath = "/shell?cmd=sleep%204%3B%20echo%20done"
const verySlowShellPath = "/shell?cmd=sleep%2015%3B%20echo%20done"

func TestIngressNGINXTimeouts(t *testing.T) {
	t.Parallel()
	t.Run("slow response allowed", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
			GatewayImplementation: implementation.IstioName,
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
