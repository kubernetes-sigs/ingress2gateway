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

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
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
			suffix, err := framework.RandString()
			require.NoError(t, err)
			regexHost := fmt.Sprintf("regex-host-%s.example.com", suffix)
			implementationSpecific := networkingv1.PathTypeImplementationSpecific
			exactPathType := networkingv1.PathTypeExact

			plain := framework.BasicIngress().
				WithName("plain").
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithHost(regexHost).
				WithPath("/hoSTn"). // Check for case-insensitivity of regex matching
				Build()
			// Exact becomes regex which are prefix
			plain.Spec.Rules[0].HTTP.Paths[0].PathType = &exactPathType

			regex := framework.BasicIngress().
				WithName("regex").
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithHost(regexHost).
				WithPath("/cliEnt.+"). // Check for case-insensitivity of regex matching
				WithAnnotation(ingressnginx.UseRegexAnnotation, "true").
				Build()
			regex.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{plain, regex},
				Verifiers: map[string][]framework.Verifier{
					"plain": {
						&framework.HTTPRequestVerifier{
							Host: regexHost,
							Path: "/hostname",
						},
					},
					"regex": {
						&framework.HTTPRequestVerifier{
							Host: regexHost,
							Path: "/clientip",
						},
					},
				},
			})
		})
		t.Run("rewrite-target implies host-level matching", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err)
			regexHost := fmt.Sprintf("rewrite-regex-host-%s.example.com", suffix)
			implementationSpecific := networkingv1.PathTypeImplementationSpecific
			exactPathType := networkingv1.PathTypeExact

			plain := framework.BasicIngress().
				WithName("plain").
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithHost(regexHost).
				WithPath("/hostn").
				Build()
			plain.Spec.Rules[0].HTTP.Paths[0].PathType = &exactPathType

			rewriteRegex := framework.BasicIngress().
				WithName("rewrite-regex").
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithHost(regexHost).
				WithPath("/client.+").
				WithAnnotation(ingressnginx.RewriteTargetAnnotation, "/").
				Build()
			rewriteRegex.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{plain, rewriteRegex},
				Verifiers: map[string][]framework.Verifier{
					"plain": {
						&framework.HTTPRequestVerifier{
							Host: regexHost,
							Path: "/hostname",
						},
					},
					"rewrite-regex": {
						&framework.HTTPRequestVerifier{
							Host: regexHost,
							Path: "/clientip",
						},
					},
				},
			})
		})
		t.Run("regex ending with dollar matches only exact path", func(t *testing.T) {
			suffix, err := framework.RandString()
			require.NoError(t, err)
			host := fmt.Sprintf("regex-dollar-%s.example.com", suffix)
			implementationSpecific := networkingv1.PathTypeImplementationSpecific

			ing := framework.BasicIngress().
				WithName("dollar").
				WithIngressClass(ingressnginx.NginxIngressClass).
				WithHost(host).
				WithPath("/$").
				WithAnnotation(ingressnginx.UseRegexAnnotation, "true").
				Build()
			ing.Spec.Rules[0].HTTP.Paths[0].PathType = &implementationSpecific

			runTestCase(t, &framework.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{ing},
				Verifiers: map[string][]framework.Verifier{
					"dollar": {
						&framework.HTTPRequestVerifier{
							Host: host,
							Path: "/",
						},
						&framework.HTTPRequestVerifier{
							Host:         host,
							Path:         "/hostname",
							AllowedCodes: []int{404},
						},
						&framework.HTTPRequestVerifier{
							Host:         host,
							Path:         "/hostname/",
							AllowedCodes: []int{404},
						},
					},
				},
			})
		})
	})
}
