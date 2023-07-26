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

package i2gw

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_constructIngressesFromFile(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	ingress1ClassName := "ingressClass1"
	ingress2ClassName := "ingressClass2"
	ingressNoNamespaceClassName := "ingressClassNoNamespace"
	ingress1 := networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "ingress1", Namespace: "namespace1"},
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
	}
	ingress2 := networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "ingress2", Namespace: "namespace2"},
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
	}
	ingressNoNamespace := networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "ingress-no-namespace"},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressNoNamespaceClassName,
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/test-no-namespace",
							PathType: &iPrefix,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "test-no-namespace",
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
	}

	testCases := []struct {
		name            string
		filePath        string
		namespace       string
		wantIngressList []networkingv1.Ingress
	}{{
		name:            "Test yaml input file with multiple resources with no namespace flag",
		filePath:        "testdata/input-file.yaml",
		namespace:       "",
		wantIngressList: []networkingv1.Ingress{ingress1, ingress2, ingressNoNamespace},
	}, {
		name:            "Test json input file with multiple resources with no namespace flag",
		filePath:        "testdata/input-file.json",
		namespace:       "",
		wantIngressList: []networkingv1.Ingress{ingress1, ingress2, ingressNoNamespace},
	}, {
		name:            "Test yaml input file with multiple resources with namespace1 flag",
		filePath:        "testdata/input-file.yaml",
		namespace:       "namespace1",
		wantIngressList: []networkingv1.Ingress{ingress1},
	},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotIngressList := &networkingv1.IngressList{}
			err := ConstructIngressesFromFile(gotIngressList, tc.filePath, tc.namespace)
			if err != nil {
				t.Errorf("Failed to open test file: %v", err)
			}
			compareIngressLists(t, gotIngressList, tc.wantIngressList)
		})
	}
}

func compareIngressLists(t *testing.T, gotIngressList *networkingv1.IngressList, wantIngressList []networkingv1.Ingress) {
	for i, got := range gotIngressList.Items {
		want := wantIngressList[i]
		if !apiequality.Semantic.DeepEqual(got, want) {
			t.Errorf("Expected Ingress %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
		}
	}
}
