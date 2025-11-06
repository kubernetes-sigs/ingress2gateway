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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_parseCanaryConfig(t *testing.T) {
	testCases := []struct {
		name           string
		ingress        networkingv1.Ingress
		expectedConfig canaryConfig
		expectError    bool
		errorContains  string
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
			expectedConfig: canaryConfig{
				weight:      50,
				weightTotal: 100,
			},
			expectError: false,
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
			expectedConfig: canaryConfig{
				weight:      50,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "weight set to 0",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "0",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      0,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "weight set to 100",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "100",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      100,
				weightTotal: 100,
			},
			expectError: false,
		},
		{
			name: "custom weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "50",
						"nginx.ingress.kubernetes.io/canary-weight-total": "200",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      50,
				weightTotal: 200,
			},
			expectError: false,
		},
		{
			name: "no weight annotation defaults to 0",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary": "true",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      0,
				weightTotal: 100,
			},
			expectError: false,
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
			expectError:   true,
			errorContains: "invalid canary-weight annotation",
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
			expectError:   true,
			errorContains: "invalid canary-weight-total annotation",
		},
		{
			name: "errors on invalid weight string",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "abc",
					},
				},
			},
			expectError:   true,
			errorContains: "invalid canary-weight annotation",
		},
		{
			name: "errors on invalid weight total string",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "xyz",
					},
				},
			},
			expectError:   true,
			errorContains: "invalid canary-weight-total annotation",
		},
		{
			name: "errors on negative weight",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":        "true",
						"nginx.ingress.kubernetes.io/canary-weight": "-10",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight must be non-negative",
		},
		{
			name: "errors on zero weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "0",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight-total must be positive",
		},
		{
			name: "errors on negative weight total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight-total": "-100",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight-total must be positive",
		},
		{
			name: "errors when weight exceeds total",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "150",
						"nginx.ingress.kubernetes.io/canary-weight-total": "100",
					},
				},
			},
			expectError:   true,
			errorContains: "canary-weight (150) exceeds canary-weight-total (100)",
		},
		{
			name: "weight equal to total is valid",
			ingress: networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":              "true",
						"nginx.ingress.kubernetes.io/canary-weight":       "200",
						"nginx.ingress.kubernetes.io/canary-weight-total": "200",
					},
				},
			},
			expectedConfig: canaryConfig{
				weight:      200,
				weightTotal: 200,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := parseCanaryConfig(&tc.ingress)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Fatalf("expected error containing %q, got %q", tc.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(config, tc.expectedConfig, cmp.AllowUnexported(canaryConfig{})); diff != "" {
				t.Fatalf("parseCanaryConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

