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
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/traefik"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// Traefik provider features: one test per annotation / annotation combination,
// all using Istio + standard emitter.

func TestTraefikTLS(t *testing.T) {
	// router.entrypoints=websecure marks the route as HTTPS-only. HTTPS traffic must reach
	// the backend via the converted gateway.
	suffix, err := framework.RandString()
	require.NoError(t, err)
	host := "traefik-tls-" + suffix + ".example.com"
	tlsSecret, err := framework.GenerateSelfSignedTLSSecret("traefik-tls-cert-"+suffix, host, []string{host})
	require.NoError(t, err)

	runTestCase(t, &framework.TestCase{
		Providers:             []string{traefik.Name},
		GatewayImplementation: implementation.IstioName,
		Secrets:               []*corev1.Secret{tlsSecret.Secret},
		Ingresses: []*networkingv1.Ingress{
			framework.BasicIngress().
				WithName("traefik-tls").
				WithHost(host).
				WithIngressClass(traefik.TraefikIngressClass).
				WithAnnotation(traefik.RouterEntrypointsAnnotation, "websecure").
				WithTLSSecret(tlsSecret.Secret.Name, host).
				Build(),
		},
		Verifiers: map[string][]framework.Verifier{
			"traefik-tls": {
				&framework.HTTPRequestVerifier{
					Host:      host,
					Path:      "/",
					UseTLS:    true,
					CACertPEM: tlsSecret.CACert,
				},
			},
		},
	})
}

func TestTraefikWebOverride(t *testing.T) {
	// router.entrypoints=web produces an HTTP-only gateway: plain HTTP traffic must reach
	// the backend with no HTTPS listener.
	runTestCase(t, &framework.TestCase{
		Providers:             []string{traefik.Name},
		GatewayImplementation: implementation.IstioName,
		Ingresses: []*networkingv1.Ingress{
			framework.BasicIngress().
				WithName("traefik-web").
				WithIngressClass(traefik.TraefikIngressClass).
				WithAnnotation(traefik.RouterEntrypointsAnnotation, "web").
				Build(),
		},
		Verifiers: map[string][]framework.Verifier{
			"traefik-web": {
				&framework.HTTPRequestVerifier{Path: "/"},
			},
		},
	})
}

func TestTraefikUnsupportedAnnotation(t *testing.T) {
	// An annotation that Traefik supports but i2gw has no Gateway API equivalent for must not
	// block conversion — traffic must still reach the backend as if the annotation were not present.
	// router.priority is valid to Traefik (affects routing priority only) but unsupported by i2gw.
	runTestCase(t, &framework.TestCase{
		Providers:             []string{traefik.Name},
		GatewayImplementation: implementation.IstioName,
		Ingresses: []*networkingv1.Ingress{
			framework.BasicIngress().
				WithName("traefik-unsupported").
				WithIngressClass(traefik.TraefikIngressClass).
				WithAnnotation("traefik.ingress.kubernetes.io/router.priority", "100").
				Build(),
		},
		Verifiers: map[string][]framework.Verifier{
			"traefik-unsupported": {
				&framework.HTTPRequestVerifier{Path: "/"},
			},
		},
	})
}
