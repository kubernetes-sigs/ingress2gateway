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

	"github.com/stretchr/testify/assert"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_implementationSpecificHTTPPathTypeMatch(t *testing.T) {
	testCases := []struct {
		name          string
		inputPath     string
		expectedType  gatewayv1.PathMatchType
		expectedValue string
	}{
		{
			name:          "regex path with wildcard",
			inputPath:     "/.*/execution/.*",
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/.*/execution/.*",
		},
		{
			name:          "regex path with specific pattern",
			inputPath:     "/api/v3/amp/login.*",
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/api/v3/amp/login.*",
		},
		{
			name:          "simple path",
			inputPath:     "/page/track.*",
			expectedType:  gatewayv1.PathMatchRegularExpression,
			expectedValue: "/page/track.*",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := &gatewayv1.HTTPPathMatch{
				Value: &tc.inputPath,
			}

			implementationSpecificHTTPPathTypeMatch(path)

			assert.NotNil(t, path.Type)
			assert.Equal(t, tc.expectedType, *path.Type)
			assert.NotNil(t, path.Value)
			assert.Equal(t, tc.expectedValue, *path.Value)
		})
	}
}
