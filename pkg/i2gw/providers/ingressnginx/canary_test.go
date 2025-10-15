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

package ingressnginx

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_parseCanaryAnnotations(t *testing.T) {
	testCases := []struct {
		name                      string
		ingress                   networkingv1.Ingress
		expectedCanaryAnnotations *canaryAnnotations
		expectedError             field.ErrorList
	}{
		{
			name: "actually get weights",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "50",
						"nginx.ingress.kubernetes.io/canary-weight-total": "100",
					},
				},
			},
			expectedCanaryAnnotations: &canaryAnnotations{
				enable:      true,
				weight:      50,
				weightTotal: 100,
			},
		},
		{
			name: "headers",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":                 "true",
						"nginx.ingress.kubernetes.io/canary-by-header":       "header",
						"nginx.ingress.kubernetes.io/canary-by-header-value": "true",
					},
				},
			},
			expectedCanaryAnnotations: &canaryAnnotations{
				enable:           true,
				headerKey:        "header",
				headerValue:      "true",
				headerRegexMatch: false,
			},
		},
		{
			name: "headers regex",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":                   "true",
						"nginx.ingress.kubernetes.io/canary-by-header":         "header",
						"nginx.ingress.kubernetes.io/canary-by-header-pattern": "abc.*",
					},
				},
			},
			expectedCanaryAnnotations: &canaryAnnotations{
				enable:           true,
				headerKey:        "header",
				headerValue:      "abc.*",
				headerRegexMatch: true,
			},
		},

		{
			name: "assigns default weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "50",
					},
				},
			},
			expectedCanaryAnnotations: &canaryAnnotations{
				enable:      true,
				weight:      50,
				weightTotal: 100,
			},
		},
		{
			name: "assigns default weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "50",
					},
				},
			},
			expectedCanaryAnnotations: &canaryAnnotations{
				enable:      true,
				weight:      50,
				weightTotal: 100,
			},
		},
		{
			name: "errors on non integer weight",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "50.5",
					},
				},
			},
			expectedCanaryAnnotations: &canaryAnnotations{},
			expectedError:             field.ErrorList{field.TypeInvalid(field.NewPath(""), "", "")},
		},
		{
			name: "errors on non integer weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "50.5",
					},
				},
			},
			expectedCanaryAnnotations: &canaryAnnotations{},
			expectedError:             field.ErrorList{field.TypeInvalid(field.NewPath(""), "", "")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			actualCanary, errs := parseCanaryAnnotations(&tc.ingress)
			if len(errs) != len(tc.expectedError) {
				t.Fatalf("expected %d errors, got %d", len(tc.expectedError), len(errs))
			}

			if len(tc.expectedError) > 0 {
				return
			}

			expectedCanary := tc.expectedCanaryAnnotations

			if diff := cmp.Diff(actualCanary, *expectedCanary, cmp.AllowUnexported(canaryAnnotations{})); diff != "" {
				t.Fatalf("getExtra() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
