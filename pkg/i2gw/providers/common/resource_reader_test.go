/*
Copyright 2024 The Kubernetes Authors.

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

package common

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_ExtractObjectsFromReader(t *testing.T) {
	ingress1 := ingress(443, "ingress1", "namespace1")
	ingress2 := ingress(80, "ingress2", "namespace2")
	ingressNoNamespace := ingress(80, "ingress-no-namespace", "")

	service1 := service(443, "https", "service1", "namespace1")
	service2 := service(80, "http", "service2", "namespace2")
	serviceNoNamespace := service(80, "http", "service-no-namespace", "")

	testCases := []struct {
		name            string
		filePath        string
		namespace       string
		wantIngressList []networkingv1.Ingress
		wantServiceList []apiv1.Service
	}{
		{
			name:            "Test yaml input file with multiple resources with no namespace flag",
			filePath:        "testdata/input-file.yaml",
			namespace:       "",
			wantIngressList: []networkingv1.Ingress{ingress1, ingress2, ingressNoNamespace},
			wantServiceList: []apiv1.Service{service1, service2, serviceNoNamespace},
		}, {
			name:            "Test json input file with multiple resources with no namespace flag",
			filePath:        "testdata/input-file.json",
			namespace:       "",
			wantIngressList: []networkingv1.Ingress{ingress1, ingress2, ingressNoNamespace},
			wantServiceList: []apiv1.Service{service1, service2, serviceNoNamespace},
		}, {
			name:            "Test yaml input file with multiple resources with namespace1 flag",
			filePath:        "testdata/input-file.yaml",
			namespace:       "namespace1",
			wantIngressList: []networkingv1.Ingress{ingress1},
			wantServiceList: []apiv1.Service{service1},
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
			gotServiceList, err := serviceListFromUnstructured(unstructuredObjects)
			if err != nil {
				t.Errorf("got unexpected error: %v", err)
			}

			compareIngressLists(t, gotIngressList, tc.wantIngressList)
			compareServiceLists(t, gotServiceList, tc.wantServiceList)
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

func service(port int32, portName string, name string, namespace string) apiv1.Service {
	var objMeta metav1.ObjectMeta
	if namespace != "" {
		objMeta = metav1.ObjectMeta{Name: name, ResourceVersion: "999", Namespace: namespace}
	} else {
		objMeta = metav1.ObjectMeta{Name: name, ResourceVersion: "999"}
	}

	service := apiv1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: objMeta,
		Spec: apiv1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []apiv1.ServicePort{{
				Name: portName,
				Port: port,
			}},
		},
	}
	return service
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

func serviceListFromUnstructured(unstructuredObjects []*unstructured.Unstructured) (*apiv1.ServiceList, error) {
	serviceList := &apiv1.ServiceList{}
	for _, f := range unstructuredObjects {
		if !f.GroupVersionKind().Empty() && f.GroupVersionKind().Kind == "Service" {
			var service apiv1.Service
			err := runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &service)
			if err != nil {
				return nil, err
			}
			serviceList.Items = append(serviceList.Items, service)
		}
	}
	return serviceList, nil
}

func compareIngressLists(t *testing.T, gotIngressList *networkingv1.IngressList, wantIngressList []networkingv1.Ingress) {
	for i, got := range gotIngressList.Items {
		want := wantIngressList[i]
		if !apiequality.Semantic.DeepEqual(got, want) {
			t.Errorf("Expected Ingress %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
		}
	}
}

func compareServiceLists(t *testing.T, gotServiceList *apiv1.ServiceList, wantServiceList []apiv1.Service) {
	for i, got := range gotServiceList.Items {
		want := wantServiceList[i]
		if !apiequality.Semantic.DeepEqual(got, want) {
			t.Errorf("Expected Service %d to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
		}
	}
}
