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

func TestIngressNGINXRedirect(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("permanent redirect", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			redirectURL := fmt.Sprintf("https://new-site-%s.example.com/new-path/", suffix)

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("permanent-redirect").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", redirectURL).
						build(),
				},
				verifiers: map[string][]verifier{
					"permanent-redirect": {
						&httpRequestVerifier{
							path: "/",
							allowedCodes: []int{
								http.StatusMovedPermanently, // 301
							},
							headerMatches: []headerMatch{
								{
									name: "Location",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
									},
								},
							},
						},
					},
				},
			})
		})

		t.Run("temporal redirect", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			redirectURL := fmt.Sprintf("https://temp-site-%s.example.com/temp-path/", suffix)

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("temporal-redirect").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", redirectURL).
						build(),
				},
				verifiers: map[string][]verifier{
					"temporal-redirect": {
						&httpRequestVerifier{
							path: "/",
							allowedCodes: []int{
								http.StatusFound, // 302
							},
							headerMatches: []headerMatch{
								{
									name: "Location",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
									},
								},
							},
						},
					},
				},
			})
		})

		t.Run("permanent redirect with supported custom code", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			redirectURL := fmt.Sprintf("https://custom-code-%s.example.com/path/", suffix)

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("permanent-redirect-301").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", redirectURL).
						withAnnotation("nginx.ingress.kubernetes.io/permanent-redirect-code", "301").
						build(),
				},
				verifiers: map[string][]verifier{
					"permanent-redirect-301": {
						&httpRequestVerifier{
							path: "/",
							allowedCodes: []int{
								http.StatusMovedPermanently, // 301
							},
							headerMatches: []headerMatch{
								{
									name: "Location",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
									},
								},
							},
						},
					},
				},
			})
		})

		t.Run("temporal redirect with supported custom code", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			redirectURL := fmt.Sprintf("https://custom-temp-%s.example.com/path/", suffix)

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("temporal-redirect-302").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", redirectURL).
						withAnnotation("nginx.ingress.kubernetes.io/temporal-redirect-code", "302").
						build(),
				},
				verifiers: map[string][]verifier{
					"temporal-redirect-302": {
						&httpRequestVerifier{
							path: "/",
							allowedCodes: []int{
								http.StatusFound, // 302
							},
							headerMatches: []headerMatch{
								{
									name: "Location",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
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

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("redirect-hostname-only").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", redirectURL).
						build(),
				},
				verifiers: map[string][]verifier{
					"redirect-hostname-only": {
						&httpRequestVerifier{
							path: "/some/path",
							allowedCodes: []int{
								http.StatusMovedPermanently, // 301
							},
							headerMatches: []headerMatch{
								{
									name: "Location",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
									},
								},
							},
						},
					},
				},
			})
		})

		t.Run("redirect with port", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			redirectURL := fmt.Sprintf("https://custom-port-%s.example.com:8443/secure/", suffix)

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("redirect-with-port").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", redirectURL).
						build(),
				},
				verifiers: map[string][]verifier{
					"redirect-with-port": {
						&httpRequestVerifier{
							path: "/",
							allowedCodes: []int{
								http.StatusFound, // 302
							},
							headerMatches: []headerMatch{
								{
									name: "Location",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(redirectURL) + "$")},
									},
								},
							},
						},
					},
				},
			})
		})

		t.Run("both redirect annotations - temporal takes priority", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			permanentURL := fmt.Sprintf("https://permanent-%s.example.com/path/", suffix)
			temporalURL := fmt.Sprintf("https://temporal-%s.example.com/path/", suffix)

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("both-redirects").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/permanent-redirect", permanentURL).
						withAnnotation("nginx.ingress.kubernetes.io/temporal-redirect", temporalURL).
						build(),
				},
				verifiers: map[string][]verifier{
					"both-redirects": {
						&httpRequestVerifier{
							path: "/",
							allowedCodes: []int{
								http.StatusFound, // 302 - temporal takes priority
							},
							headerMatches: []headerMatch{
								{
									name: "Location",
									patterns: []*maybeNegativePattern{
										{pattern: regexp.MustCompile("^" + regexp.QuoteMeta(temporalURL) + "$")},
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
