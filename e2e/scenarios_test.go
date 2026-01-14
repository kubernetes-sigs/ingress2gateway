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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	networkingv1 "k8s.io/api/networking/v1"
)

// NOTE: Setting the Host field in ingress rules and in verifiers is optional. When omitted, a
// random host is generated and used automatically for all ingress objects and verifiers in the
// test case. Most test cases likely don't need an explicit Host value since the value doesn't
// matter as long as the verifier verifies the correct Host. In case a specific Host value is
// important for some test cases, it's important to pay attention to duplicate Host values across
// test cases: While k8s allows defining multiple ingress objects with identical Host values,
// whether doing so makes sense (or even works) depends on the ingress controller and can influence
// test results.

func TestIngressNginx(t *testing.T) {
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
						withName("foo").
						withIngressClass(ingressnginx.NginxIngressClass).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {&httpGetVerifier{path: "/"}},
				},
			})
		})
		t.Run("with host field", func(t *testing.T) {
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
						withName("foo").
						withIngressClass(ingressnginx.NginxIngressClass).
						withHost("foo.example.com").
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							host: "foo.example.com",
							path: "/",
						},
					},
				},
			})
		})
		t.Run("multiple ingresses", func(t *testing.T) {
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
						withName("foo").
						withIngressClass(ingressnginx.NginxIngressClass).
						build(),
					basicIngress().
						withName("bar").
						withIngressClass(ingressnginx.NginxIngressClass).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {&httpGetVerifier{path: "/"}},
					"bar": {&httpGetVerifier{path: "/"}},
				},
			})
		})
	})
	// TODO: The Cilium implementation requires Cilium to be the cluster CNI. To run Cilium tests,
	// create a kind cluster with disableDefaultCNI: true.
	// t.Run("to Cilium", func(t *testing.T) {
	//  t.Parallel()
	// 	t.Run("basic conversion", func(t *testing.T) {
	// 		runTestCase(t, &testCase{
	// 			gatewayImplementation: cilium.Name,
	// 			...
	// 		})
	// 	})
	// })
}

func TestKongIngress(t *testing.T) {
	t.Parallel()
	t.Run("to Kong Gateway", func(t *testing.T) {
		t.Parallel()
		t.Run("basic conversion", func(t *testing.T) {
			runTestCase(t, &testCase{
				gatewayImplementation: kong.Name,
				providers:             []string{kong.Name},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("foo").
						withIngressClass(kong.KongIngressClass).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {&httpGetVerifier{path: "/"}},
				},
			})
		})
		t.Run("multiple ingresses", func(t *testing.T) {
			runTestCase(t, &testCase{
				gatewayImplementation: kong.Name,
				providers:             []string{kong.Name},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("foo").
						withIngressClass(kong.KongIngressClass).
						build(),
					basicIngress().
						withName("bar").
						withIngressClass(kong.KongIngressClass).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {&httpGetVerifier{path: "/"}},
					"bar": {&httpGetVerifier{path: "/"}},
				},
			})
		})
	})
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("basic conversion", func(t *testing.T) {
			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{kong.Name},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("foo").
						withIngressClass(kong.KongIngressClass).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {&httpGetVerifier{path: "/"}},
				},
			})
		})
		t.Run("multiple ingresses", func(t *testing.T) {
			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{kong.Name},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("foo").
						withIngressClass(kong.KongIngressClass).
						build(),
					basicIngress().
						withName("bar").
						withIngressClass(kong.KongIngressClass).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {&httpGetVerifier{path: "/"}},
					"bar": {&httpGetVerifier{path: "/"}},
				},
			})
		})
	})
}

func TestMultipleProviders(t *testing.T) {
	t.Parallel()
	t.Run("ingress-nginx + kong", func(t *testing.T) {
		t.Parallel()
		t.Run("to Istio", func(t *testing.T) {
			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name, kong.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					basicIngress().
						withName("foo").
						withIngressClass(ingressnginx.NginxIngressClass).
						build(),
					basicIngress().
						withName("bar").
						withIngressClass(kong.KongIngressClass).
						build(),
				},
				verifiers: map[string][]verifier{
					"foo": {&httpGetVerifier{path: "/"}},
					"bar": {&httpGetVerifier{path: "/"}},
				},
			})
		})
	})
}
