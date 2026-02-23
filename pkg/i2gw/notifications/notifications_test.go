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

package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestObjectsToStr(t *testing.T) {
	testCases := []struct {
		name    string
		objects []client.Object
		want    string
	}{
		{
			name: "single object",
			objects: []client.Object{
				&networkingv1.Ingress{
					TypeMeta: metav1.TypeMeta{
						Kind: "Ingress",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "single-ingress",
						Namespace: "test",
					},
				},
			},
			want: "Ingress: test/single-ingress",
		},
		{
			name: "two objects",
			objects: []client.Object{
				&gatewayv1.Gateway{
					TypeMeta: metav1.TypeMeta{
						Kind: "Gateway",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "way",
						Namespace: "gate",
					},
				},
				&gatewayv1.HTTPRoute{
					TypeMeta: metav1.TypeMeta{
						Kind: "HTTPRoute",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route",
						Namespace: "prod",
					},
				},
			},
			want: "Gateway: gate/way, HTTPRoute: prod/route",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := objectsToStr(tc.objects)
			assert.Equal(t, tc.want, result)
		})
	}
}
