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
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestIngressNGINXRegex(t *testing.T) {
	t.Parallel()

	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("host-level matching", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			regexHost := fmt.Sprintf("regex-host-%s.example.com", suffix)
			implementationSpecific := networkingv1.PathTypeImplementationSpecific
			exactPathType := networkingv1.PathTypeExact

			plain := basicIngress().
				withName("plain").
				withIngressClass(ingressnginx.NginxIngressClass).
				withHost(regexHost).
				withPath("/hoSTn"). // Check for case-insensitivity of regex matching
				build()
			// Exact becomes regex which are prefix
			plain.Spec.Rules[0].HTTP.Paths[0].PathType = &exactPathType

			regex := basicIngress().
				withName("regex").
				withIngressClass(ingressnginx.NginxIngressClass).
				withHost(regexHost).
				withPath("/cliEnt.+"). // Check for case-insensitivity of regex matching
				withAnnotation(ingressnginx.UseRegexAnnotation, "true").
				build()
			regex.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{plain, regex},
				verifiers: map[string][]verifier{
					"plain": {
						&httpRequestVerifier{
							host: regexHost,
							path: "/hostname",
						},
					},
					"regex": {
						&httpRequestVerifier{
							host: regexHost,
							path: "/clientip",
						},
					},
				},
			})
		})
		t.Run("rewrite-target implies host-level matching", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			regexHost := fmt.Sprintf("rewrite-regex-host-%s.example.com", suffix)
			implementationSpecific := networkingv1.PathTypeImplementationSpecific
			exactPathType := networkingv1.PathTypeExact

			plain := basicIngress().
				withName("plain").
				withIngressClass(ingressnginx.NginxIngressClass).
				withHost(regexHost).
				withPath("/hostn").
				build()
			plain.Spec.Rules[0].HTTP.Paths[0].PathType = &exactPathType

			rewriteRegex := basicIngress().
				withName("rewrite-regex").
				withIngressClass(ingressnginx.NginxIngressClass).
				withHost(regexHost).
				withPath("/client.+").
				withAnnotation(ingressnginx.RewriteTargetAnnotation, "/").
				build()
			rewriteRegex.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{plain, rewriteRegex},
				verifiers: map[string][]verifier{
					"plain": {
						&httpRequestVerifier{
							host: regexHost,
							path: "/hostname",
						},
					},
					"rewrite-regex": {
						&httpRequestVerifier{
							host: regexHost,
							path: "/clientip",
						},
					},
				},
			})
		})
		t.Run("regex ending with dollar matches only exact path", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err)
			host := fmt.Sprintf("regex-dollar-%s.example.com", suffix)
			implementationSpecific := networkingv1.PathTypeImplementationSpecific

			ing := basicIngress().
				withName("dollar").
				withIngressClass(ingressnginx.NginxIngressClass).
				withHost(host).
				withPath("/$").
				withAnnotation(ingressnginx.UseRegexAnnotation, "true").
				build()
			ing.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{ing},
				verifiers: map[string][]verifier{
					"dollar": {
						&httpRequestVerifier{
							host: host,
							path: "/",
						},
						&httpRequestVerifier{
							host:         host,
							path:         "/hostname",
							allowedCodes: []int{404},
						},
						&httpRequestVerifier{
							host:         host,
							path:         "/hostname/",
							allowedCodes: []int{404},
						},
					},
				},
			})
		})
	})
}
