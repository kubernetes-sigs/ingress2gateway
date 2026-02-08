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
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/stretchr/testify/assert"
)

var ingressText = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-with-matching-ingressclass
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
  name: ingress-without-matching-ingressclass
spec:
  ingressClassName: ingress-nginx
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
var IngressClass = "nginx"

// Test that the ingress-class provider-specific flag is honored by the resource reader
func TestResourceReader_FiltersByIngressClass_FromFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "ingress.yaml")

	f, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer f.Close()

	if _, err = io.WriteString(f, ingressText); err != nil {
		t.Fatalf("failed to write string: %v", err)
	}

	// Configure the ingress-nginx provider with the ingress-class flag set to "nginx".
	conf := &i2gw.ProviderConf{
		ProviderSpecificFlags: map[string]map[string]string{
			Name: {
				NginxIngressClassFlag: IngressClass,
			},
		},
	}

	rr := newResourceReader(conf)
	storage, err := rr.readResourcesFromFile(filePath)
	if err != nil {
		t.Fatalf("readResourcesFromFile() error = %v", err)
	}

	ingresses := storage.Ingresses.List()

	assert.Len(t, ingresses, 1, "Expected exactly one ingress to be selected")
	assert.Equal(t, IngressClass, *ingresses[0].Spec.IngressClassName, "Ingresses fetched should have the provided ingressClass")
}
