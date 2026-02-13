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
	"k8s.io/utils/ptr"
)

func TestIngressNGINXPathMatching(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()
		t.Run("path matching", func(t *testing.T) {
			ingressForPath := func(name, path string, pathType networkingv1.PathType) *networkingv1.Ingress {
				ing := basicIngress().
					withName(name).
					withIngressClass(ingressnginx.NginxIngressClass).
					withPath(path).
					build()
				ing.Spec.Rules[0].HTTP.Paths[0].PathType = ptr.To(pathType)
				return ing
			}

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				ingresses: []*networkingv1.Ingress{
					ingressForPath("exact", "/hostname", networkingv1.PathTypeExact),
					ingressForPath("prefix", "/hostname", networkingv1.PathTypePrefix),
					ingressForPath("exact-slash", "/hostname/", networkingv1.PathTypeExact),
					ingressForPath("prefix-slash", "/hostname/", networkingv1.PathTypePrefix),
				},
				verifiers: map[string][]verifier{
					"exact": {
						&httpRequestVerifier{path: "/hostname", allowedCodes: []int{200}},
						&httpRequestVerifier{path: "/hostname/", allowedCodes: []int{404}},
						&httpRequestVerifier{path: "/hostnameaaa", allowedCodes: []int{404}},
					},
					"prefix": {
						&httpRequestVerifier{path: "/hostname", allowedCodes: []int{200}},
						&httpRequestVerifier{path: "/hostname/", allowedCodes: []int{200}},
						&httpRequestVerifier{path: "/hostnameaaa", allowedCodes: []int{404}},
					},
					"exact-slash": {
						&httpRequestVerifier{
							path:         "/hostname",
							allowedCodes: []int{301},
							headerMatches: []headerMatch{{
								name: "Location",
								patterns: []*maybeNegativePattern{
									{pattern: regexp.MustCompile("/hostname/$")},
								},
							}},
						},
						&httpRequestVerifier{path: "/hostname/", allowedCodes: []int{200}},
						&httpRequestVerifier{path: "/hostnameaaa", allowedCodes: []int{404}},
					},
					"prefix-slash": {
						&httpRequestVerifier{
							path:         "/hostname",
							allowedCodes: []int{301},
							headerMatches: []headerMatch{{
								name: "Location",
								patterns: []*maybeNegativePattern{
									{pattern: regexp.MustCompile("/hostname/$")},
								},
							}},
						},
						&httpRequestVerifier{path: "/hostname/", allowedCodes: []int{200}},
						&httpRequestVerifier{path: "/hostnameaaa", allowedCodes: []int{404}},
					},
				},
			})
		})
	})
}
