/*
Copyright 2026 The Kubernetes Authors.

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

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_implementationSpecificHTTPPathTypeMatch(t *testing.T) {
	testCases := []struct {
		name          string
		inputPath     string
		annotations   map[string]string
		expectedType  gatewayv1.PathMatchType
		expectedValue string
	}{
		{
			name:      "regex path with use-regex annotation",
			inputPath: "/.*/execution/.*",
			annotations: map[string]string{
				UseRegexAnnotation: "true",
			},
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/.*/execution/.*",
		},
		{
			name:      "regex path with use-regex annotation set to true",
			inputPath: "/api/v3/amp/login.*",
			annotations: map[string]string{
				UseRegexAnnotation: "true",
			},
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/api/v3/amp/login.*",
		},
		{
			name:      "use-regex with value 1 (strconv.ParseBool)",
			inputPath: "/api/v1/.*",
			annotations: map[string]string{
				UseRegexAnnotation: "1",
			},
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/api/v1/.*",
		},
		{
			name:      "use-regex with value TRUE (strconv.ParseBool)",
			inputPath: "/api/v2/.*",
			annotations: map[string]string{
				UseRegexAnnotation: "TRUE",
			},
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/api/v2/.*",
		},
		{
			name:      "use-regex with value t (strconv.ParseBool)",
			inputPath: "/api/v3/.*",
			annotations: map[string]string{
				UseRegexAnnotation: "t",
			},
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/api/v3/.*",
		},
		{
			name:        "path without use-regex annotation defaults to Prefix",
			inputPath:   "/page/track",
			annotations: map[string]string{
				// No use-regex annotation
			},
			expectedType:  gatewayv1.PathMatchPathPrefix,
			expectedValue: "/page/track",
		},
		{
			name:      "path with use-regex set to false defaults to Prefix",
			inputPath: "/api/v1/users",
			annotations: map[string]string{
				UseRegexAnnotation: "false",
			},
			expectedType:  gatewayv1.PathMatchPathPrefix,
			expectedValue: "/api/v1/users",
		},
		{
			name:      "use-regex with value 0 (strconv.ParseBool false)",
			inputPath: "/api/v2/users",
			annotations: map[string]string{
				UseRegexAnnotation: "0",
			},
			expectedType:  gatewayv1.PathMatchPathPrefix,
			expectedValue: "/api/v2/users",
		},
		{
			name:      "use-regex with invalid value defaults to Prefix",
			inputPath: "/api/v3/users",
			annotations: map[string]string{
				UseRegexAnnotation: "invalid",
			},
			expectedType:  gatewayv1.PathMatchPathPrefix,
			expectedValue: "/api/v3/users",
		},
		{
			name:          "path with nil annotations defaults to Prefix",
			inputPath:     "/api/v2/products",
			annotations:   nil,
			expectedType:  gatewayv1.PathMatchPathPrefix,
			expectedValue: "/api/v2/products",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := &gatewayv1.HTTPPathMatch{
				Value: &tc.inputPath,
			}

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}

			implementationSpecificHTTPPathTypeMatch(path, []networkingv1.Ingress{*ingress})

			assert.NotNil(t, path.Type)
			assert.Equal(t, tc.expectedType, *path.Type)
			assert.NotNil(t, path.Value)
			assert.Equal(t, tc.expectedValue, *path.Value)
		})
	}
}

func Test_parseBoolAnnotation(t *testing.T) {
	testCases := []struct {
		name           string
		annotations    map[string]string
		annotationKey  string
		expectedResult bool
	}{
		{
			name: "annotation with value true",
			annotations: map[string]string{
				"test-annotation": "true",
			},
			annotationKey:  "test-annotation",
			expectedResult: true,
		},
		{
			name: "annotation with value 1",
			annotations: map[string]string{
				"test-annotation": "1",
			},
			annotationKey:  "test-annotation",
			expectedResult: true,
		},
		{
			name: "annotation with value TRUE",
			annotations: map[string]string{
				"test-annotation": "TRUE",
			},
			annotationKey:  "test-annotation",
			expectedResult: true,
		},
		{
			name: "annotation with value t",
			annotations: map[string]string{
				"test-annotation": "t",
			},
			annotationKey:  "test-annotation",
			expectedResult: true,
		},
		{
			name: "annotation with value false",
			annotations: map[string]string{
				"test-annotation": "false",
			},
			annotationKey:  "test-annotation",
			expectedResult: false,
		},
		{
			name: "annotation with value 0",
			annotations: map[string]string{
				"test-annotation": "0",
			},
			annotationKey:  "test-annotation",
			expectedResult: false,
		},
		{
			name: "annotation with invalid value",
			annotations: map[string]string{
				"test-annotation": "invalid",
			},
			annotationKey:  "test-annotation",
			expectedResult: false,
		},
		{
			name:           "annotation not present",
			annotations:    map[string]string{},
			annotationKey:  "test-annotation",
			expectedResult: false,
		},
		{
			name:           "nil annotations",
			annotations:    nil,
			annotationKey:  "test-annotation",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}
			result := parseBoolAnnotation(ingress, tc.annotationKey)
			assert.Equal(t, tc.expectedResult, result)
		})
	}

	// Test with nil ingress
	t.Run("nil ingress", func(t *testing.T) {
		result := parseBoolAnnotation(nil, "test-annotation")
		assert.False(t, result)
	})
}

func Test_isCanary(t *testing.T) {
	testCases := []struct {
		name           string
		annotations    map[string]string
		expectedResult bool
	}{
		{
			name: "canary with value true",
			annotations: map[string]string{
				CanaryAnnotation: "true",
			},
			expectedResult: true,
		},
		{
			name: "canary with value 1",
			annotations: map[string]string{
				CanaryAnnotation: "1",
			},
			expectedResult: true,
		},
		{
			name: "canary with value TRUE",
			annotations: map[string]string{
				CanaryAnnotation: "TRUE",
			},
			expectedResult: true,
		},
		{
			name: "canary with value t",
			annotations: map[string]string{
				CanaryAnnotation: "t",
			},
			expectedResult: true,
		},
		{
			name: "canary with value false",
			annotations: map[string]string{
				CanaryAnnotation: "false",
			},
			expectedResult: false,
		},
		{
			name: "canary with value 0",
			annotations: map[string]string{
				CanaryAnnotation: "0",
			},
			expectedResult: false,
		},
		{
			name: "canary with invalid value",
			annotations: map[string]string{
				CanaryAnnotation: "invalid",
			},
			expectedResult: false,
		},
		{
			name:           "canary annotation not present",
			annotations:    map[string]string{},
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}
			result := isCanary(ingress)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func Test_implementationSpecificHTTPPathTypeMatch_withCanary(t *testing.T) {
	testCases := []struct {
		name         string
		inputPath    string
		ingresses    []networkingv1.Ingress
		expectedType gatewayv1.PathMatchType
	}{
		{
			name:      "canary inherits use-regex from main - main has regex",
			inputPath: "/api/.*",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "canary",
						Annotations: map[string]string{
							CanaryAnnotation: "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "main",
						Annotations: map[string]string{
							UseRegexAnnotation: "true",
						},
					},
				},
			},
			expectedType: gatewayv1.PathMatchRegularExpression,
		},
		{
			name:      "canary has use-regex but main doesn't - should use main (no regex)",
			inputPath: "/api/v1",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "canary",
						Annotations: map[string]string{
							CanaryAnnotation:   "true",
							UseRegexAnnotation: "true", // This should be ignored
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "main",
						Annotations: map[string]string{},
					},
				},
			},
			expectedType: gatewayv1.PathMatchPathPrefix,
		},
		{
			name:      "all canaries - should default to prefix",
			inputPath: "/api/v2",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "canary1",
						Annotations: map[string]string{
							CanaryAnnotation: "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "canary2",
						Annotations: map[string]string{
							CanaryAnnotation: "true",
						},
					},
				},
			},
			expectedType: gatewayv1.PathMatchPathPrefix,
		},
		{
			name:      "canary with value false - should be treated as main",
			inputPath: "/api/v3",
			ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-really-canary",
						Annotations: map[string]string{
							CanaryAnnotation: "false",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "actual-main",
						Annotations: map[string]string{
							UseRegexAnnotation: "true",
						},
					},
				},
			},
			expectedType: gatewayv1.PathMatchPathPrefix,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := &gatewayv1.HTTPPathMatch{
				Value: &tc.inputPath,
			}

			implementationSpecificHTTPPathTypeMatch(path, tc.ingresses)

			assert.NotNil(t, path.Type)
			assert.Equal(t, tc.expectedType, *path.Type)
		})
	}
}
