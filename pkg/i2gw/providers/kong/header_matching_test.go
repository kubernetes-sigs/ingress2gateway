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

func TestHeaderMatchingFeature(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix

	testCases := []struct {
		name                     string
		ingresses                []networkingv1.Ingress
		expectedHTTPRouteMatches map[string][][]gatewayv1.HTTPRouteMatch
		expectedErrors           field.ErrorList
	}{
		{
			name: "header matching - ORed headers",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ored-headers",
						Namespace: "default",
						Annotations: map[string]string{
							"konghq.com/headers.key": "val1,val2",
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
				"default/ored-headers-test-mydomain-com": {
					{
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "key",
									Value: "val1",
								},
							},
						},
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "key",
									Value: "val2",
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "header matching - ANDed/ORed headers",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "anded-ored-headers",
						Namespace: "default",
						Annotations: map[string]string{
							"konghq.com/headers.keyA": "val1,val2",
							"konghq.com/headers.keyB": "val3,val4,val5",
							"konghq.com/headers.keyC": "val6",
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
				"default/anded-ored-headers-test-mydomain-com": {
					{
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "keyA",
									Value: "val1",
								},
								{
									Name:  "keyB",
									Value: "val3",
								},
								{
									Name:  "keyC",
									Value: "val6",
								},
							},
						},
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "keyA",
									Value: "val1",
								},
								{
									Name:  "keyB",
									Value: "val4",
								},
								{
									Name:  "keyC",
									Value: "val6",
								},
							},
						},
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "keyA",
									Value: "val1",
								},
								{
									Name:  "keyB",
									Value: "val5",
								},
								{
									Name:  "keyC",
									Value: "val6",
								},
							},
						},
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "keyA",
									Value: "val2",
								},
								{
									Name:  "keyB",
									Value: "val3",
								},
								{
									Name:  "keyC",
									Value: "val6",
								},
							},
						},
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "keyA",
									Value: "val2",
								},
								{
									Name:  "keyB",
									Value: "val4",
								},
								{
									Name:  "keyC",
									Value: "val6",
								},
							},
						},
						gatewayv1.HTTPRouteMatch{
							Headers: []gatewayv1.HTTPHeaderMatch{
								{
									Name:  "keyA",
									Value: "val2",
								},
								{
									Name:  "keyB",
									Value: "val5",
								},
								{
									Name:  "keyC",
									Value: "val6",
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
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

			errs = headerMatchingFeature(tc.ingresses, &gatewayResources)
			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
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
