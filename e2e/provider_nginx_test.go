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
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// NGINX Ingress Controller provider features: One test per feature, all using Istio + standard
// emitter.

// nginxProviderFlags selects the ingress class used by NGINX Ingress Controller in the e2e tests.
var nginxProviderFlags = map[string]map[string]string{
	nginx.Name: {
		nginx.NginxIngressClassFlag: framework.NginxIngressClass,
	},
}

func TestNginxProxySetHeaders(t *testing.T) {
	t.Parallel()
	t.Run("static header is set on upstream request", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithIngressClass(framework.NginxIngressClass).
					WithAnnotation("nginx.org/proxy-set-headers", "X-I2GW-Test: set-by-nginx").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					// agnhost echoes the requested header value back on /header?key=...
					&framework.HTTPRequestVerifier{
						Path:      "/header?key=X-I2GW-Test",
						BodyRegex: regexp.MustCompile(`^set-by-nginx$`),
					},
				},
			},
		})
	})
}

func TestNginxProxyHideHeaders(t *testing.T) {
	t.Parallel()
	t.Run("upstream response header is removed", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithIngressClass(framework.NginxIngressClass).
					WithAnnotation("nginx.org/proxy-hide-headers", "X-Content-Type-Options").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					// agnhost replies with http.Error (which sets X-Content-Type-Options) when
					// /dial is requested without a request parameter.
					&framework.HTTPRequestVerifier{
						Path:         "/dial",
						AllowedCodes: []int{http.StatusBadRequest},
						HeaderAbsent: []string{"X-Content-Type-Options"},
					},
				},
			},
		})
	})
}

func TestNginxRewrites(t *testing.T) {
	t.Parallel()
	t.Run("prefix is rewritten for the matching service", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithIngressClass(framework.NginxIngressClass).
					WithPath("/app").
					WithAnnotation("nginx.org/rewrites", fmt.Sprintf("serviceName=%s rewrite=/hostname", framework.DummyAppName1)).
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					// /app is rewritten to /hostname. Only /hostname returns the pod name: any
					// other path is served by agnhost's root handler.
					&framework.HTTPRequestVerifier{
						Path:      "/app",
						BodyRegex: regexp.MustCompile("^" + framework.DummyAppName1),
					},
				},
			},
		})
	})
}

func TestNginxPathRegex(t *testing.T) {
	t.Parallel()
	t.Run("exact", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithIngressClass(framework.NginxIngressClass).
					WithPath("/hostname").
					WithAnnotation("nginx.org/path-regex", "exact").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					&framework.HTTPRequestVerifier{
						Path:      "/hostname",
						BodyRegex: regexp.MustCompile("^" + framework.DummyAppName1),
					},
					// A Prefix match would route this request. An Exact match must not.
					&framework.HTTPRequestVerifier{
						Path:         "/hostname/extra",
						AllowedCodes: []int{http.StatusNotFound},
					},
				},
			},
		})
	})
	t.Run("case sensitive regex", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithIngressClass(framework.NginxIngressClass).
					WithPath("/host[n]ame").
					WithAnnotation("nginx.org/path-regex", "case_sensitive").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					&framework.HTTPRequestVerifier{
						Path:      "/hostname",
						BodyRegex: regexp.MustCompile("^" + framework.DummyAppName1),
					},
					// The regex doesn't match, so nothing should be routed.
					&framework.HTTPRequestVerifier{
						Path:         "/hostxame",
						AllowedCodes: []int{http.StatusNotFound},
					},
				},
			},
		})
	})
}

func TestNginxHSTS(t *testing.T) {
	t.Parallel()
	t.Run("hsts header is set on TLS responses", func(t *testing.T) {
		suffix, err := framework.RandString()
		require.NoError(t, err, "creating host suffix")
		host := "hsts-" + suffix + ".example.com"
		tlsSecret, err := framework.GenerateSelfSignedTLSSecret("hsts-"+suffix, host, []string{host})
		require.NoError(t, err, "creating TLS secret")

		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Secrets:               []*corev1.Secret{tlsSecret.Secret},
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithHost(host).
					WithIngressClass(framework.NginxIngressClass).
					WithTLSSecret(tlsSecret.Secret.Name, host).
					WithAnnotation("nginx.org/hsts", "true").
					WithAnnotation("nginx.org/hsts-max-age", "3600").
					WithAnnotation("nginx.org/hsts-include-subdomains", "true").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					// NGINX Ingress Controller also appends "preload", so only match the
					// directives that ingress2gateway converts.
					&framework.HTTPRequestVerifier{
						Host:      host,
						Path:      "/",
						UseTLS:    true,
						CACertPEM: tlsSecret.CACert,
						HeaderMatches: []framework.HeaderMatch{
							{
								Name: "Strict-Transport-Security",
								Patterns: []*framework.MaybeNegativePattern{
									{Pattern: regexp.MustCompile(`max-age=3600`)},
									{Pattern: regexp.MustCompile(`includeSubDomains`)},
								},
							},
						},
					},
				},
			},
		})
	})
}

func TestNginxClientMaxBodySize(t *testing.T) {
	t.Parallel()
	// The standard emitter has no Gateway API equivalent for client-max-body-size, so the body
	// size limit is dropped and only routing is expected to work. See
	// TestNginxEmitterClientSettingsPolicy for the NGINX Gateway Fabric specific conversion.
	t.Run("routing still works with the standard emitter", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.IstioName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithIngressClass(framework.NginxIngressClass).
					WithAnnotation("nginx.org/client-max-body-size", "1k").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					&framework.HTTPRequestVerifier{
						Path:      "/hostname",
						BodyRegex: regexp.MustCompile("^" + framework.DummyAppName1),
					},
				},
			},
		})
	})
}
