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

package ingressnginx

import (
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	networkingv1 "k8s.io/api/networking/v1"
)


func TestPathRewrite(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("basic conversion", func(t *testing.T) {
			e2e.RunTestCase(t, &e2e.TestCase{
				GatewayImplementation: istio.ProviderName,
				Providers:             []string{ingressnginx.Name},
				ProviderFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				Ingresses: []*networkingv1.Ingress{
					e2e.BasicIngress().
						WithName("foo1").
						WithIngressClass(ingressnginx.NginxIngressClass).
						WithPath("/abc").
						WithAnnotation("nginx.ingress.kubernetes.io/rewrite-target", "/header").
						WithAnnotation("nginx.ingress.kubernetes.io/x-forwarded-prefix", "/abc").
						Build(),
				},
				Verifiers: map[string][]e2e.Verifier{
					"foo1": {
						&e2e.HttpGetVerifier{Path: "/abc", BodyIncludes: []string{`"X-Forwarded-Prefix":["/abc"]`}},
					},
				},
			})
		})
	})
}
