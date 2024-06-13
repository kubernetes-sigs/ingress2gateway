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

package gce

import (
	"testing"
)

func TestIngClassToGwyClassGCE(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc                 string
		ingressClass         string
		expectedGatewayClass string
		expectedNil          bool
	}{
		{
			desc:                 "gce ingress class",
			ingressClass:         gceIngressClass,
			expectedGatewayClass: gceL7GlobalExternalManagedGatewayClass,
			expectedNil:          true,
		},
		{
			desc:                 "gce-internal ingress class",
			ingressClass:         gceL7ILBIngressClass,
			expectedGatewayClass: gceL7RegionalInternalGatewayClass,
			expectedNil:          true,
		},
		{
			desc:                 "unexpected ingress class",
			ingressClass:         "unexpected",
			expectedGatewayClass: "",
			expectedNil:          false,
		},
		{
			desc:                 "missing ingress class",
			ingressClass:         "",
			expectedGatewayClass: gceL7GlobalExternalManagedGatewayClass,
			expectedNil:          true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotGatewayClass, gotErr := ingClassToGwyClassGCE(tc.ingressClass)
			if gotGatewayClass != tc.expectedGatewayClass {
				t.Errorf("ingClassToGwyClassGCE() = %v, expected %v", gotGatewayClass, tc.expectedGatewayClass)
			}
			gotNil := gotErr == nil
			if gotNil != tc.expectedNil {
				t.Errorf("ingClassToGwyClassGCE() = %v, expected %v", gotNil, tc.expectedNil)
			}
		})
	}
}
