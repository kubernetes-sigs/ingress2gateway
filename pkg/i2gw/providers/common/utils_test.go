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
	networkingv1 "k8s.io/api/networking/v1"
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
					networkingv1.IngressRule{
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
						},
					},
				},
			},
		},
		{
			name: "1 rule, multiple matches, different path",
			rules: []ingressRule{
				{
					networkingv1.IngressRule{
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
						},
					},
				},
			},
		},
		{
			name: "multiple rules with single matches, same path",
			rules: []ingressRule{
				{
					networkingv1.IngressRule{
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
				},
				{
					networkingv1.IngressRule{
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
						},
					},
				},
			},
		},
		{
			name: "multiple rules with single matches, different path",
			rules: []ingressRule{
				{
					networkingv1.IngressRule{
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
				},
				{
					networkingv1.IngressRule{
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
						},
					},
				},
			},
		},
		{
			name: "multiple rules with multiple matches, mixed paths",
			rules: []ingressRule{
				{
					networkingv1.IngressRule{
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
				},
				{
					networkingv1.IngressRule{
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
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			want := groupIngressPathsByMatchKey(tc.rules)
			require.Equal(t, want, tc.expected)
		})
	}
}
