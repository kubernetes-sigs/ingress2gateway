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
	networkingv1 "k8s.io/api/networking/v1"
)

func TestIngressNGINXRedirect(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
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
	})
}
