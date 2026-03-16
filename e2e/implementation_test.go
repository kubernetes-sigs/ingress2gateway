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
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	networkingv1 "k8s.io/api/networking/v1"
)

// Implementation smoke tests: Table-driven tests across gateway implementations, all using
// ingress-nginx provider + standard emitter.

func TestImplementations(t *testing.T) {
	t.Parallel()

	implementations := []struct {
		name string
	}{
		{name: implementation.IstioName},
		{name: implementation.KongName},
		{name: implementation.KgatewayName},
		{name: implementation.EnvoyGatewayName},
		{name: implementation.AgentgatewayName},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			t.Parallel()
			t.Run("basic conversion", func(t *testing.T) {
				runTestCase(t, &framework.TestCase{
					GatewayImplementation: impl.name,
					Providers:             []string{ingressnginx.Name},
					Ingresses: []*networkingv1.Ingress{
						framework.BasicIngress().
							WithName("foo").
							WithIngressClass(ingressnginx.NginxIngressClass).
							Build(),
					},
					Verifiers: map[string][]framework.Verifier{
						"foo": {&framework.HTTPRequestVerifier{Path: "/"}},
					},
				})
			})
			t.Run("multiple ingresses", func(t *testing.T) {
				runTestCase(t, &framework.TestCase{
					GatewayImplementation: impl.name,
					Providers:             []string{ingressnginx.Name},
					Ingresses: []*networkingv1.Ingress{
						framework.BasicIngress().
							WithName("foo").
							WithIngressClass(ingressnginx.NginxIngressClass).
							Build(),
						framework.BasicIngress().
							WithName("bar").
							WithIngressClass(ingressnginx.NginxIngressClass).
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
}
