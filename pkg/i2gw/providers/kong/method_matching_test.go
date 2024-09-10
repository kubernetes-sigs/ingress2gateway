/*
Copyright 2023 The Kubernetes Authors.

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

package kong

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestMethodMatchingFeature(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix

	testCases := []struct {
		name                     string
		ingresses                []networkingv1.Ingress
		expectedHTTPRouteMatches map[string][][]gatewayv1.HTTPRouteMatch
		expectedErrors           field.ErrorList
	}{
		{
			name: "method matching - 1 method",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "one-method",
						Namespace: "default",
						Annotations: map[string]string{
							"konghq.com/methods": "GET",
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("ingress-kong"),
						Rules: []networkingv1.IngressRule{{
							Host: "test.mydomain.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			expectedHTTPRouteMatches: map[string][][]gatewayv1.HTTPRouteMatch{
				"default/one-method-test-mydomain-com": {
					{
						gatewayv1.HTTPRouteMatch{
							Method: ptrTo(gatewayv1.HTTPMethodGet),
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "method matching - many methods",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "many-methods",
						Namespace: "default",
						Annotations: map[string]string{
							"konghq.com/methods": "GET,POST,DELETE",
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("ingress-kong"),
						Rules: []networkingv1.IngressRule{{
							Host: "test.mydomain.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			expectedHTTPRouteMatches: map[string][][]gatewayv1.HTTPRouteMatch{
				"default/many-methods-test-mydomain-com": {
					{
						gatewayv1.HTTPRouteMatch{
							Method: ptrTo(gatewayv1.HTTPMethodGet),
						},
						gatewayv1.HTTPRouteMatch{
							Method: ptrTo(gatewayv1.HTTPMethodPost),
						},
						gatewayv1.HTTPRouteMatch{
							Method: ptrTo(gatewayv1.HTTPMethodDelete),
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "method matching - wrong method",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-method",
						Namespace: "default",
						Annotations: map[string]string{
							"konghq.com/methods": "WRONG",
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: ptrTo("ingress-kong"),
						Rules: []networkingv1.IngressRule{{
							Host: "test.mydomain.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			expectedErrors: field.ErrorList{
				field.Invalid(
					field.NewPath("default/wrong-method-wrong-method").Child("metadata").Child("annotations"),
					"WRONG",
					"method not supported",
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gatewayResources, errs := common.ToIR(tc.ingresses, i2gw.ProviderImplementationSpecificOptions{
				ToImplementationSpecificHTTPPathTypeMatch: implementationSpecificHTTPPathTypeMatch,
			})
			if len(errs) != 0 {
				t.Errorf("Expected no errors, got %d: %+v", len(errs), errs)
			}

			errs = methodMatchingFeature(tc.ingresses, &gatewayResources)
			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
				if len(errs) > 0 {
					return
				}
			}

			for _, httpRoute := range gatewayResources.HTTPRoutes {
				keyName := fmt.Sprintf("%s/%s", httpRoute.Namespace, httpRoute.Name)
				for i, rule := range httpRoute.Spec.Rules {
					if !containsMatches(rule.Matches, tc.expectedHTTPRouteMatches[keyName][i]) {
						t.Errorf("Expected %+v matches, got %+v", tc.expectedHTTPRouteMatches, rule.Matches)
					}
				}
			}
		})
	}
}
