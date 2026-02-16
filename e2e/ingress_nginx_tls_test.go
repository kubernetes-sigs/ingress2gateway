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
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

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
