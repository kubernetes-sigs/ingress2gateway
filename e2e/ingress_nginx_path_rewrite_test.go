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
	networkingv1 "k8s.io/api/networking/v1"
)

func TestPathRewrite(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("basic conversion", func(t *testing.T) {
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
						WithName("foo1").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithPath("/abc").
						WithAnnotation("nginx.ingress.kubernetes.io/rewrite-target", "/header").
						WithAnnotation("nginx.ingress.kubernetes.io/x-forwarded-prefix", "/abc").
						Build(),
				},
				verifiers: map[string][]verifier{
					"foo1": {
						&httpGetVerifier{path: "/abc", bodyRegex: regexp.MustCompile(`"X-Forwarded-Prefix":\["/abc"\]`)},
					},
				},
			})
		})
	})
}
