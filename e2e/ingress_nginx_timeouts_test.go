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
	"net/http"
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	networkingv1 "k8s.io/api/networking/v1"
)

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
