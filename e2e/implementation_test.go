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
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	networkingv1 "k8s.io/api/networking/v1"
)

// Implementation smoke tests: Table-driven test covering all gateway implementations. Each
// implementation uses the ingress-nginx provider + standard emitter to isolate implementation
// behavior from provider behavior.

func TestImplementationSmoke(t *testing.T) {
	t.Parallel()

	type implEntry struct {
		implementation string
		provider       string
		ingressClass   string
		providerFlags  map[string]map[string]string
	}

	implementations := []implEntry{
		{
			implementation: implementation.KongName,
			provider:       ingressnginx.Name,
			ingressClass:   ingressnginx.NginxIngressClass,
		},
		{
			implementation: implementation.IstioName,
			provider:       ingressnginx.Name,
			ingressClass:   ingressnginx.NginxIngressClass,
		},
		{
			implementation: implementation.KgatewayName,
			provider:       ingressnginx.Name,
			ingressClass:   ingressnginx.NginxIngressClass,
		},
	}

	for _, impl := range implementations {
		t.Run(impl.implementation, func(t *testing.T) {
			t.Parallel()
			t.Run("basic conversion", func(t *testing.T) {
				runTestCase(t, &framework.TestCase{
					GatewayImplementation: impl.implementation,
					Providers:             []string{impl.provider},
					ProviderFlags:         impl.providerFlags,
					Ingresses: []*networkingv1.Ingress{
						framework.BasicIngress().
							WithName("foo").
							WithIngressClass(impl.ingressClass).
							Build(),
					},
					Verifiers: map[string][]framework.Verifier{
						"foo": {&framework.HTTPRequestVerifier{Path: "/"}},
					},
				})
			})
			t.Run("multiple ingresses", func(t *testing.T) {
				runTestCase(t, &framework.TestCase{
					GatewayImplementation: impl.implementation,
					Providers:             []string{impl.provider},
					ProviderFlags:         impl.providerFlags,
					Ingresses: []*networkingv1.Ingress{
						framework.BasicIngress().
							WithName("foo").
							WithIngressClass(impl.ingressClass).
							Build(),
						framework.BasicIngress().
							WithName("bar").
							WithIngressClass(impl.ingressClass).
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
