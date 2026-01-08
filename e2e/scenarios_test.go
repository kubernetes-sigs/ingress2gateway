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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(ingressnginx.NginxIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							path: "/",
						},
					},
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(ingressnginx.NginxIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									Host: "foo.example.com",
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(ingressnginx.NginxIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bar",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(ingressnginx.NginxIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							path: "/",
						},
					},
					"bar": {
						&httpGetVerifier{
							path: "/",
						},
					},
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(kong.KongIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							path: "/",
						},
					},
				},
			})
		})
		t.Run("multiple ingresses", func(t *testing.T) {
			runTestCase(t, &testCase{
				gatewayImplementation: kong.Name,
				providers:             []string{kong.Name},
				ingresses: []*networkingv1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(kong.KongIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bar",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(kong.KongIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							path: "/",
						},
					},
					"bar": {
						&httpGetVerifier{
							path: "/",
						},
					},
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(kong.KongIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							path: "/",
						},
					},
				},
			})
		})
		t.Run("multiple ingresses", func(t *testing.T) {
			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{kong.Name},
				ingresses: []*networkingv1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(kong.KongIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bar",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(kong.KongIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							path: "/",
						},
					},
					"bar": {
						&httpGetVerifier{
							path: "/",
						},
					},
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(ingressnginx.NginxIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bar",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To(kong.KongIngressClass),
							Rules: []networkingv1.IngressRule{
								{
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				verifiers: map[string][]verifier{
					"foo": {
						&httpGetVerifier{
							path: "/",
						},
					},
					"bar": {
						&httpGetVerifier{
							path: "/",
						},
					},
				},
			})
		})
	})
}
