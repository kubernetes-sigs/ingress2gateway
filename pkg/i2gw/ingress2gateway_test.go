/*
Copyright 2022 The Kubernetes Authors.

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
package i2gw

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_inputFile(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	ingress1ClassName := "ingress1-example"
	ingress2ClassName := "ingress2-example"

	expectIngresses := []networkingv1.Ingress{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Ingress",
				APIVersion: "networking.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{Name: "ingress1"},
			Spec: networkingv1.IngressSpec{
				IngressClassName: &ingress1ClassName,
				Rules: []networkingv1.IngressRule{{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/test-1",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test1",
										Port: networkingv1.ServiceBackendPort{
											Number: 443,
										},
									},
								},
							}},
						},
					},
				}},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Ingress",
				APIVersion: "networking.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{Name: "ingress2"},
			Spec: networkingv1.IngressSpec{
				IngressClassName: &ingress2ClassName,
				Rules: []networkingv1.IngressRule{{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/test-2",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test2",
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
	}

	testCases := []struct {
		name            string
		filePath        string
		expectIngresses []networkingv1.Ingress
	}{{
		name:            "Test yaml input file with multiple resources",
		filePath:        "testdata/input-file.yaml",
		expectIngresses: expectIngresses,
	}, {
		name:            "Test json input file with multiple resources",
		filePath:        "testdata/input-file.json",
		expectIngresses: expectIngresses,
	},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingressList := &networkingv1.IngressList{}
			err := constructIngressesFromFile(ingressList, tc.filePath)
			if err != nil {
				t.Errorf("Failed to open test file: %v", err)
			}
			for i, got := range ingressList.Items {
				want := expectIngresses[i]
				if !apiequality.Semantic.DeepEqual(got, want) {
					t.Errorf("Expected Ingress %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
				}
			}
		})
	}
}
