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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_constructIngressesFromFile(t *testing.T) {
	ingress1 := ingress(443, "ingress1", "namespace1")
	ingress2 := ingress(80, "ingress2", "namespace2")
	ingressNoNamespace := ingress(80, "ingress-no-namespace", "")

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

func ingress(port int32, name, namespace string) networkingv1.Ingress {
	iPrefix := networkingv1.PathTypePrefix
	ingressClassName := fmt.Sprintf("ingressClass-%s", name)
	var objMeta metav1.ObjectMeta
	if namespace != "" {
		objMeta = metav1.ObjectMeta{Name: name, ResourceVersion: "999", Namespace: namespace}
	} else {
		objMeta = metav1.ObjectMeta{Name: name, ResourceVersion: "999"}
	}

	ing := networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: objMeta,
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     fmt.Sprintf("/path-%s", name),
							PathType: &iPrefix,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: fmt.Sprintf("service-%s", name),
									Port: networkingv1.ServiceBackendPort{
										Number: port,
									},
								},
							},
						}},
					},
				},
			}},
		},
	}
	return ing
}

func compareIngressLists(t *testing.T, gotIngressList *networkingv1.IngressList, wantIngressList []networkingv1.Ingress) {
	for i, got := range gotIngressList.Items {
		want := wantIngressList[i]
		if !apiequality.Semantic.DeepEqual(got, want) {
			t.Errorf("Expected Ingress %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
		}
	}
}

func Test_constructIngressesFromCluster(t *testing.T) {
	ingress1 := ingress(443, "ingress1", "namespace1")
	ingress2 := ingress(80, "ingress2", "namespace2")
	testCases := []struct {
		name          string
		runtimeObjs   []runtime.Object
		wantIngresses []networkingv1.Ingress
	}{{
		name:          "Test cluster client with 2 resources",
		runtimeObjs:   []runtime.Object{&ingress1, &ingress2},
		wantIngresses: []networkingv1.Ingress{ingress1, ingress2},
	}, {
		name:          "Test cluster client without resources",
		runtimeObjs:   []runtime.Object{},
		wantIngresses: []networkingv1.Ingress{},
	},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotIngresses := &networkingv1.IngressList{}
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.runtimeObjs...).Build()
			err := ConstructIngressesFromCluster(cl, gotIngresses)
			if err != nil {
				t.Errorf("test failed unexpectedly: %v", err)
			}
			compareIngressLists(t, gotIngresses, tc.wantIngresses)
		})
	}
}
