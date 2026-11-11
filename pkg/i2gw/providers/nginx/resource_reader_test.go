/*
Copyright The Kubernetes Authors.

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

package nginx

import (
	"strings"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ingressesWithMixedClasses = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-nginx-class
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: test
            port:
              number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-custom-class
spec:
  ingressClassName: nginx-ic
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: test
            port:
              number: 80
`

func TestResourceReader_FiltersByIngressClass_FromFile(t *testing.T) {
	tests := []struct {
		name        string
		flags       map[string]map[string]string
		wantIngress string
	}{
		{
			name:        "defaults to nginx when flag is not set",
			flags:       nil,
			wantIngress: "ingress-nginx-class",
		},
		{
			name: "defaults to nginx when flag is empty",
			flags: map[string]map[string]string{
				Name: {NginxIngressClassFlag: ""},
			},
			wantIngress: "ingress-nginx-class",
		},
		{
			name: "honors ingress-class flag",
			flags: map[string]map[string]string{
				Name: {NginxIngressClassFlag: "nginx-ic"},
			},
			wantIngress: "ingress-custom-class",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := newResourceReader(&i2gw.ProviderConf{ProviderSpecificFlags: tt.flags})

			storage, err := rr.readResourcesFromFile(strings.NewReader(ingressesWithMixedClasses))
			require.NoError(t, err)

			require.Len(t, storage.Ingresses, 1, "expected exactly one ingress to be selected")
			for key := range storage.Ingresses {
				assert.Equal(t, tt.wantIngress, key.Name)
			}
		})
	}
}
