/*
Copyright 2025 The Kubernetes Authors.

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
				useRegexAnnotation: "true",
			},
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/.*/execution/.*",
		},
		{
			name:      "regex path with use-regex annotation set to true",
			inputPath: "/api/v3/amp/login.*",
			annotations: map[string]string{
				useRegexAnnotation: "true",
			},
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/api/v3/amp/login.*",
		},
		{
			name:      "path without use-regex annotation defaults to Prefix",
			inputPath: "/page/track",
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
				useRegexAnnotation: "false",
			},
			expectedType:  gatewayv1.PathMatchPathPrefix,
			expectedValue: "/api/v1/users",
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

			implementationSpecificHTTPPathTypeMatch(path, ingress)

			assert.NotNil(t, path.Type)
			assert.Equal(t, tc.expectedType, *path.Type)
			assert.NotNil(t, path.Value)
			assert.Equal(t, tc.expectedValue, *path.Value)
		})
	}
}
