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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/traefik"
	networkingv1 "k8s.io/api/networking/v1"
)

// Provider basics: Table-driven tests across providers, all using Istio + standard emitter.

func TestProviders(t *testing.T) {
	t.Parallel()

	providers := []struct {
		name          string
		ingressClass  string
		providerFlags map[string]map[string]string
	}{
		{
			name:         ingressnginx.Name,
			ingressClass: ingressnginx.NginxIngressClass,
			providerFlags: map[string]map[string]string{
				ingressnginx.Name: {
					ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
				},
			},
		},
		{
			name:         kong.Name,
			ingressClass: kong.KongIngressClass,
		},
		{
			name:         traefik.Name,
			ingressClass: traefik.TraefikIngressClass,
		},
	}

	for _, prov := range providers {
		t.Run(prov.name, func(t *testing.T) {
			t.Parallel()
			t.Run("to Istio", func(t *testing.T) {
				t.Parallel()
				t.Run("basic conversion", func(t *testing.T) {
					runTestCase(t, &framework.TestCase{
						GatewayImplementation: implementation.IstioName,
						Providers:             []string{prov.name},
						ProviderFlags:         prov.providerFlags,
						Ingresses: []*networkingv1.Ingress{
							framework.BasicIngress().
								WithName("foo").
								WithIngressClass(prov.ingressClass).
								Build(),
						},
						Verifiers: map[string][]framework.Verifier{
							"foo": {&framework.HTTPRequestVerifier{Path: "/"}},
						},
					})
				})
				t.Run("multiple ingresses", func(t *testing.T) {
					runTestCase(t, &framework.TestCase{
						GatewayImplementation: implementation.IstioName,
						Providers:             []string{prov.name},
						ProviderFlags:         prov.providerFlags,
						Ingresses: []*networkingv1.Ingress{
							framework.BasicIngress().
								WithName("foo").
								WithIngressClass(prov.ingressClass).
								Build(),
							framework.BasicIngress().
								WithName("bar").
								WithIngressClass(prov.ingressClass).
								Build(),
						},
						Verifiers: map[string][]framework.Verifier{
							"foo": {&framework.HTTPRequestVerifier{Path: "/"}},
							"bar": {&framework.HTTPRequestVerifier{Path: "/"}},
						},
					})
				})
			})
		})
	}
}
