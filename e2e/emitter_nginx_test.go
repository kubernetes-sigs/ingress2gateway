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

package e2e

import (
	"bytes"
	"net/http"
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx"
	networkingv1 "k8s.io/api/networking/v1"
)

// nginx emitter features: One test per feature, all using NGINX Gateway Fabric + nginx emitter.

const nginxEmitterName = "nginx"

func TestNginxEmitterClientSettingsPolicy(t *testing.T) {
	t.Parallel()
	t.Run("client-max-body-size produces ClientSettingsPolicy", func(t *testing.T) {
		runTestCase(t, &framework.TestCase{
			GatewayImplementation: implementation.NginxGatewayFabricName,
			Providers:             []string{nginx.Name},
			ProviderFlags:         nginxProviderFlags,
			Emitter:               nginxEmitterName,
			Ingresses: []*networkingv1.Ingress{
				framework.BasicIngress().
					WithName("foo").
					WithIngressClass(framework.NginxIngressClass).
					WithAnnotation("nginx.org/client-max-body-size", "1k").
					Build(),
			},
			Verifiers: map[string][]framework.Verifier{
				"foo": {
					// A body below the limit is accepted.
					&framework.HTTPRequestVerifier{
						Path:        "/echo?msg=ok",
						Method:      http.MethodPost,
						RequestBody: bytes.Repeat([]byte("a"), 512),
						BodyRegex:   regexp.MustCompile("^ok$"),
					},
					// A body above the limit is rejected by NGINX before reaching the backend.
					&framework.HTTPRequestVerifier{
						Path:         "/echo?msg=ok",
						Method:       http.MethodPost,
						RequestBody:  bytes.Repeat([]byte("a"), 2048),
						AllowedCodes: []int{http.StatusRequestEntityTooLarge},
					},
				},
			},
		})
	})
}
