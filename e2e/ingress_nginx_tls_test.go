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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestTLS(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("tls ingress and gateway", func(t *testing.T) {
			suffix, err := e2e.RandString(6)
			if err != nil {
				t.Fatalf("creating host suffix: %v", err)
			}
			host := "tls-" + suffix + ".example.com"
			tlsSecret, err := e2e.GenerateTLSTestSecret("tls-cert-"+suffix, host)
			if err != nil {
				t.Fatalf("creating TLS secret: %v", err)
			}
			e2e.RunTestCase(t, &e2e.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Secrets: []*corev1.Secret{tlsSecret.Secret},
				Ingresses: []*networkingv1.Ingress{
					e2e.BasicIngress().
						WithName("foo").
						WithHost(host).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithTLSSecret(tlsSecret.Secret.Name, host).
						Build(),
				},
				Verifiers: map[string][]e2e.Verifier{
					"foo": {
						&e2e.HttpGetVerifier{
							Host:      host,
							Path:      "/",
							UseTLS:    true,
							CACertPEM: tlsSecret.CACert,
						},
						&e2e.HttpGetVerifier{
							Host:         host,
							Path:         "/",
							AllowedCodes: []int{308},
							UseTLS:       false,
							HeaderMatches: []e2e.HeaderMatch{
								{
									Name:     "Location",
									Patterns: []*regexp.Regexp{regexp.MustCompile("^https://" + host + "/?$")},
								},
							},
						},
					},
				},
			})
		})
		t.Run("ssl-redirect annotation", func(t *testing.T) {
			suffix, err := e2e.RandString(6)
			if err != nil {
				t.Fatalf("creating host suffix: %v", err)
			}
			redirectHost := "tls-redirect-" + suffix + ".example.com"
			noRedirectHost := "tls-noredirect-" + suffix + ".example.com"
			redirectSecret, err := e2e.GenerateTLSTestSecret("tls-redirect-"+suffix, redirectHost)
			if err != nil {
				t.Fatalf("creating redirect TLS secret: %v", err)
			}
			noRedirectSecret, err := e2e.GenerateTLSTestSecret("tls-noredirect-"+suffix, noRedirectHost)
			if err != nil {
				t.Fatalf("creating no-redirect TLS secret: %v", err)
			}
			e2e.RunTestCase(t, &e2e.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Secrets: []*corev1.Secret{redirectSecret.Secret, noRedirectSecret.Secret},
				Ingresses: []*networkingv1.Ingress{
					e2e.BasicIngress().
						WithName("redirect").
						WithHost(redirectHost).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithTLSSecret(redirectSecret.Secret.Name, redirectHost).
						Build(),
					e2e.BasicIngress().
						WithName("no-redirect").
						WithHost(noRedirectHost).
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithAnnotation("nginx.ingress.kubernetes.io/ssl-redirect", "false").
						WithTLSSecret(noRedirectSecret.Secret.Name, noRedirectHost).
						Build(),
				},
				Verifiers: map[string][]e2e.Verifier{
					"redirect": {
						&e2e.HttpGetVerifier{
							Host:      redirectHost,
							Path:      "/",
							UseTLS:    true,
							CACertPEM: redirectSecret.CACert,
						},
						&e2e.HttpGetVerifier{
							Host:         redirectHost,
							Path:         "/",
							AllowedCodes: []int{308},
							UseTLS:       false,
							HeaderMatches: []e2e.HeaderMatch{
								{
									Name:     "Location",
									Patterns: []*regexp.Regexp{regexp.MustCompile("^https://" + redirectHost + "/?$")},
								},
							},
						},
					},
					"no-redirect": {
						&e2e.HttpGetVerifier{
							Host:      noRedirectHost,
							Path:      "/",
							UseTLS:    true,
							CACertPEM: noRedirectSecret.CACert,
						},
						&e2e.HttpGetVerifier{
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
