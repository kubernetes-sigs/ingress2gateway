/*
Copyright 2025 The Kubernetes Authors.

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
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	networkingv1 "k8s.io/api/networking/v1"
)

// Multi-provider tests: Tests which combine multiple providers in a single conversion.

func TestMultipleProviders(t *testing.T) {
	t.Parallel()
	t.Run("ingress-nginx + kong", func(t *testing.T) {
		t.Parallel()
		t.Run("to Istio", func(t *testing.T) {
			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name, kong.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					framework.BasicIngress().
						WithName("foo").
						WithIngressClass(ingressnginx.NginxIngressClass).
						Build(),
					framework.BasicIngress().
						WithName("bar").
						WithIngressClass(kong.KongIngressClass).
						Build(),
				},
				Verifiers: map[string][]framework.Verifier{
					"foo": {&framework.HTTPRequestVerifier{Path: "/"}},
					"bar": {&framework.HTTPRequestVerifier{Path: "/"}},
				},
			})
		})
	})
}
