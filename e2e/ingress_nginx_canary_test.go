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
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestIngressNGINXCanary(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("base canary", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("canary-%s.com", suffix)
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
						withName("foo1").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						build(),
					basicIngress().
						withName("foo2").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/canary", "true").
						withAnnotation("nginx.ingress.kubernetes.io/canary-weight", "20").
						withBackend(DummyAppName2).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo1": {
						&canaryVerifier{
							verifier: &httpRequestVerifier{
								host:      host,
								path:      "/hostname",
								bodyRegex: regexp.MustCompile("^dummy-app2"),
							},
							runs:         200,
							minSuccesses: 0.1,
							maxSuccesses: 0.3,
						},
					},
				},
			})
		})

		t.Run("canary by header at path", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("canary-header-path-%s.com", suffix)
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
						withName("main").
						withHost(host).
						withPath("/hostname").
						withIngressClass(ingressnginx.NginxIngressClass).
						withBackend(DummyAppName1).
						build(),
					basicIngress().
						withName("canary-header").
						withHost(host).
						withPath("/hostname").
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/canary", "true").
						withAnnotation("nginx.ingress.kubernetes.io/canary-by-header", "X-Canary").
						withBackend(DummyAppName2).
						build(),
				},
				verifiers: map[string][]verifier{
					"main": {
						// With the canary header set to "always", all requests at the
						// canary path should go to the canary backend.
						&httpRequestVerifier{
							host: host,
							path: "/hostname",
							requestHeaders: map[string]string{
								"X-Canary": "always",
							},
							bodyRegex: regexp.MustCompile("^dummy-app2"),
						},
						// Without the header, requests should go to the main backend.
						&httpRequestVerifier{
							host:      host,
							path:      "/hostname",
							bodyRegex: regexp.MustCompile("^dummy-app1"),
						},
					},
				},
			})
		})

		t.Run("canary weight and header combined", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("canary-combined-%s.com", suffix)
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
						withName("prod").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						withBackend(DummyAppName1).
						build(),
					basicIngress().
						withName("canary-combined").
						withHost(host).
						withIngressClass(ingressnginx.NginxIngressClass).
						withAnnotation("nginx.ingress.kubernetes.io/canary", "true").
						withAnnotation("nginx.ingress.kubernetes.io/canary-weight", "20").
						withAnnotation("nginx.ingress.kubernetes.io/canary-by-header", "X-Canary").
						withBackend(DummyAppName2).
						build(),
				},
				verifiers: map[string][]verifier{
					"prod": {
						// With the canary header set to "always", 100% of requests
						// should go to the canary backend regardless of weight.
						&httpRequestVerifier{
							host: host,
							path: "/hostname",
							requestHeaders: map[string]string{
								"X-Canary": "always",
							},
							bodyRegex: regexp.MustCompile("^dummy-app2"),
						},
						// With the canary header set to "never", 0% of requests
						// should go to the canary backend regardless of weight.
						&httpRequestVerifier{
							host: host,
							path: "/hostname",
							requestHeaders: map[string]string{
								"X-Canary": "never",
							},
							bodyRegex: regexp.MustCompile("^dummy-app1"),
						},
						// Without any header, the canary-weight (20%) applies.
						&canaryVerifier{
							verifier: &httpRequestVerifier{
								host:      host,
								path:      "/hostname",
								bodyRegex: regexp.MustCompile("^dummy-app2"),
							},
							runs:         200,
							minSuccesses: 0.1,
							maxSuccesses: 0.3,
						},
					},
				},
			})
		})
	})
}
