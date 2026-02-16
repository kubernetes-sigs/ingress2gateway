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

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
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
