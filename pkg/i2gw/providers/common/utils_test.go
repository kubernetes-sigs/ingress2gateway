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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGroupIngressPathsByMatchKey(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix

	testCases := []struct {
		name     string
		rules    []ingressRule
		expected orderedIngressPathsByMatchKey
	}{
		{
			name:  "no rules",
			rules: []ingressRule{},
			expected: orderedIngressPathsByMatchKey{
				keys: []pathMatchKey{},
				data: map[pathMatchKey][]ingressPath{},
			},
		},
		{
			name: "1 rule with 1 match",
			rules: []ingressRule{
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test",
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
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
			},
			expected: orderedIngressPathsByMatchKey{
				keys: []pathMatchKey{
					"Prefix//test",
				},
				data: map[pathMatchKey][]ingressPath{
					"Prefix//test": {
						{
							ruleIdx:  0,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test",
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
				},
			},
		},
		{
			name: "1 rule, multiple matches, different path",
			rules: []ingressRule{
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test1",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test1",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									},
									{
										Path:     "/test2",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test2",
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
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
			},
			expected: orderedIngressPathsByMatchKey{
				keys: []pathMatchKey{
					"Prefix//test1",
					"Prefix//test2",
				},
				data: map[pathMatchKey][]ingressPath{
					"Prefix//test1": {
						{
							ruleIdx:  0,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test1",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test1",
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
					"Prefix//test2": {
						{
							ruleIdx:  0,
							pathIdx:  1,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test2",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test2",
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
				},
			},
		},
		{
			name: "multiple rules with single matches, same path",
			rules: []ingressRule{
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test",
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
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test",
												Port: networkingv1.ServiceBackendPort{
													Number: 81,
												},
											},
										},
									},
								},
							},
						},
					},
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
			},
			expected: orderedIngressPathsByMatchKey{
				keys: []pathMatchKey{
					"Prefix//test",
				},
				data: map[pathMatchKey][]ingressPath{
					"Prefix//test": {
						{
							ruleIdx:  0,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test",
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
						{
							ruleIdx:  1,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test",
										Port: networkingv1.ServiceBackendPort{
											Number: 81,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
				},
			},
		},
		{
			name: "multiple rules with single matches, different path",
			rules: []ingressRule{
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test",
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
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test2",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test2",
												Port: networkingv1.ServiceBackendPort{
													Number: 81,
												},
											},
										},
									},
								},
							},
						},
					},
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
			},
			expected: orderedIngressPathsByMatchKey{
				keys: []pathMatchKey{
					"Prefix//test",
					"Prefix//test2",
				},
				data: map[pathMatchKey][]ingressPath{
					"Prefix//test": {
						{
							ruleIdx:  0,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test",
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
					"Prefix//test2": {
						{
							ruleIdx:  1,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test2",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test2",
										Port: networkingv1.ServiceBackendPort{
											Number: 81,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
				},
			},
		},
		{
			name: "multiple rules with multiple matches, mixed paths",
			rules: []ingressRule{
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test11",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test11",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									},
									{
										Path:     "/test12",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test12",
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
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
				{
					rule: networkingv1.IngressRule{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/test21",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test21",
												Port: networkingv1.ServiceBackendPort{
													Number: 81,
												},
											},
										},
									},
									{
										Path:     "/test11",
										PathType: PtrTo(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test11",
												Port: networkingv1.ServiceBackendPort{
													Number: 81,
												},
											},
										},
									},
								},
							},
						},
					},
					sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
				},
			},
			expected: orderedIngressPathsByMatchKey{
				keys: []pathMatchKey{
					"Prefix//test11",
					"Prefix//test12",
					"Prefix//test21",
				},
				data: map[pathMatchKey][]ingressPath{
					"Prefix//test11": {
						{
							ruleIdx:  0,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test11",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test11",
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
						{
							ruleIdx:  1,
							pathIdx:  1,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test11",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test11",
										Port: networkingv1.ServiceBackendPort{
											Number: 81,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
					"Prefix//test12": {
						{
							ruleIdx:  0,
							pathIdx:  1,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test12",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test12",
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
					"Prefix//test21": {
						{
							ruleIdx:  1,
							pathIdx:  0,
							ruleType: "http",
							path: networkingv1.HTTPIngressPath{
								Path:     "/test21",
								PathType: &iPrefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test21",
										Port: networkingv1.ServiceBackendPort{
											Number: 81,
										},
									},
								},
							},
							sourceIngress: types.NamespacedName{Namespace: "test", Name: "test"},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, groupIngressPathsByMatchKey(tc.rules))
		})
	}
}

func TestGroupServicePortsByPortName(t *testing.T) {
	t.Run("group service ports by port name", func(t *testing.T) {
		services := map[types.NamespacedName]*apiv1.Service{
			{Namespace: "namespace1", Name: "service1"}: {
				TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Namespace: "namespace1", Name: "service1"},
				Spec: apiv1.ServiceSpec{
					Type: "ClusterIP",
					Ports: []apiv1.ServicePort{
						{Name: "http", Port: 80},
						{Name: "https", Port: 443},
					},
				},
			},
			{Namespace: "namespace2", Name: "service2"}: {
				TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Namespace: "namespace2", Name: "service2"},
				Spec: apiv1.ServiceSpec{
					Type: "ClusterIP",
					Ports: []apiv1.ServicePort{
						{Name: "http", Port: 80},
					},
				},
			},
			{Namespace: "namespace1", Name: "service3"}: {
				TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Namespace: "namespace1", Name: "service3"},
				Spec: apiv1.ServiceSpec{
					Type: "ClusterIP",
					Ports: []apiv1.ServicePort{
						{Name: "http", Port: 9200},
						{Name: "transport", Port: 9300},
					},
				},
			},
		}
		expected := map[types.NamespacedName]map[string]int32{
			{Namespace: "namespace1", Name: "service1"}: {"http": 80, "https": 443},
			{Namespace: "namespace2", Name: "service2"}: {"http": 80},
			{Namespace: "namespace1", Name: "service3"}: {"http": 9200, "transport": 9300},
		}

		require.Equal(t, expected, GroupServicePortsByPortName(services))
	})
}
