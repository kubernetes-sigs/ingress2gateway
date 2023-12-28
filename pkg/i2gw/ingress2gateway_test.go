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
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_ExtractObjectsFromReader(t *testing.T) {
	ingress1 := ingress(443, "ingress1", "namespace1")
	ingress2 := ingress(80, "ingress2", "namespace2")
	ingressNoNamespace := ingress(80, "ingress-no-namespace", "")

	testCases := []struct {
		name            string
		filePath        string
		namespace       string
		wantIngressList []networkingv1.Ingress
	}{
		{
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
			stream, err := os.ReadFile(tc.filePath)
			if err != nil {
				t.Errorf("failed to read file %s: %v", tc.filePath, err)
			}
			unstructuredObjects, err := ExtractObjectsFromReader(bytes.NewReader(stream), tc.namespace)
			if err != nil {
				t.Errorf("failed to extract objects: %s", err)
			}
			gotIngressList, err := ingressListFromUnstructured(unstructuredObjects)
			if err != nil {
				t.Errorf("got unexpected error: %v", err)
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

func ingressListFromUnstructured(unstructuredObjects []*unstructured.Unstructured) (*networkingv1.IngressList, error) {
	ingressList := &networkingv1.IngressList{}
	for _, f := range unstructuredObjects {
		if !f.GroupVersionKind().Empty() && f.GroupVersionKind().Kind == "Ingress" {
			var i networkingv1.Ingress
			err := runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &i)
			if err != nil {
				return nil, err
			}
			ingressList.Items = append(ingressList.Items, i)
		}
	}
	return ingressList, nil
}
func compareIngressLists(t *testing.T, gotIngressList *networkingv1.IngressList, wantIngressList []networkingv1.Ingress) {
	for i, got := range gotIngressList.Items {
		want := wantIngressList[i]
		if !apiequality.Semantic.DeepEqual(got, want) {
			t.Errorf("Expected Ingress %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
		}
	}
}

func Test_constructProviders(t *testing.T) {
	supportProviders := []string{"ingress-nginx"}
	for _, provider := range supportProviders {
		ProviderConstructorByName[ProviderName(provider)] = func(conf *ProviderConf) Provider { return nil }
	}
	testCases := []struct {
		name              string
		providers         []string
		expectedProviders []string
		expectedError     error
	}{{
		name:              "Test construct providers with default providers",
		providers:         GetSupportedProviders(),
		expectedProviders: supportProviders,
		expectedError:     nil,
	}, {
		name:              "Test construct providers with provider that not supported",
		providers:         []string{"ingress-nginx", "fake-provider"},
		expectedProviders: []string{},
		expectedError:     fmt.Errorf("%s is not a supported provider", "fake-provider"),
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects([]runtime.Object{}...).Build()
			providerByName, err := constructProviders(&ProviderConf{
				Client: cl,
			}, tc.providers)
			if tc.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tc.expectedError.Error() != err.Error() {
					t.Errorf("The got error '%s' not equal to expected error '%s'", err.Error(), tc.expectedError.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got %v", err)
				}
				if len(tc.expectedProviders) != len(providerByName) {
					t.Errorf("Expected contructed providers num is %d but got %d", len(tc.providers), len(providerByName))
				}
				for _, provider := range tc.expectedProviders {
					if _, ok := providerByName[ProviderName(provider)]; !ok {
						t.Errorf("The expected provider %s was not constructed", provider)
					}
				}
			}
		})
	}
}

func Test_GetSupportedProviders(t *testing.T) {
	supportProviders := []string{"ingress-nginx"}
	for _, provider := range supportProviders {
		ProviderConstructorByName[ProviderName(provider)] = func(conf *ProviderConf) Provider { return nil }
	}
	t.Run("Test GetSupportedProviders", func(t *testing.T) {
		allProviders := GetSupportedProviders()
		if len(allProviders) != len(ProviderConstructorByName) {
			t.Errorf("The acutal number of the providers we supported is %d but we got the number is: %d",
				len(ProviderConstructorByName), len(allProviders))
		}
		for _, provider := range allProviders {
			providerName := ProviderName(provider)
			if _, ok := ProviderConstructorByName[providerName]; !ok {
				t.Errorf("%s is not a supported provider", providerName)
			}
		}
	})
}
