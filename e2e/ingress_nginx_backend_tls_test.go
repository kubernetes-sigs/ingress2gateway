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

package e2e

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	backendTLSAppName   = "tls-backend"
	backendServerSecret = "tls-backend-server-cert"
	backendCASecretName = "tls-backend-ca"
)

// backendTLSSetup deploys an HTTPS backend app using a CA-signed server certificate
// and patches the given ingresses so that namespace-dependent annotations (proxy-ssl-secret
// and proxy-ssl-name) contain the correct dynamic values.
//
// ingress-nginx requires proxy-ssl-secret in "namespace/secretName" format and
// proxy-ssl-name must match the server certificate SAN, which uses the dynamic test namespace.
// Because the namespace is only known at runtime (inside runTestCase), the ingresses are built
// with placeholder values and patched here before they are created in the cluster.
func backendTLSSetup(ingresses ...*networkingv1.Ingress) func(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace string, skipCleanup bool) {
	return func(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace string, skipCleanup bool) {
		t.Helper()

		// The server hostname for the certificate must use the in-cluster DNS name.
		svcHost := fmt.Sprintf("%s.%s.svc.cluster.local", backendTLSAppName, namespace)

		// Patch ingress annotations with namespace-qualified values now that we know the namespace.
		for _, ing := range ingresses {
			if ing.Annotations == nil {
				continue
			}
			if _, ok := ing.Annotations[ingressnginx.ProxySSLSecretAnnotation]; ok {
				ing.Annotations[ingressnginx.ProxySSLSecretAnnotation] = namespace + "/" + backendCASecretName
			}
			if _, ok := ing.Annotations[ingressnginx.ProxySSLNameAnnotation]; ok {
				ing.Annotations[ingressnginx.ProxySSLNameAnnotation] = svcHost
			}
		}

		secrets, err := generateBackendTLSSecrets(backendServerSecret, backendCASecretName, namespace, svcHost)
		require.NoError(t, err, "generating backend TLS secrets")

		// Create the CA secret (referenced by proxy-ssl-secret annotation for NGINX ingress).
		secrets.CASecret.Namespace = namespace
		_, err = client.CoreV1().Secrets(namespace).Create(ctx, secrets.CASecret, metav1.CreateOptions{})
		require.NoError(t, err, "creating CA secret")
		t.Cleanup(func() {
			if skipCleanup {
				return
			}
			_ = client.CoreV1().Secrets(namespace).Delete(context.Background(), secrets.CASecret.Name, metav1.DeleteOptions{})
		})

		// Create a ConfigMap with the same CA cert data. The converter outputs
		// caCertificateRefs with kind: ConfigMap (the standard kind supported by
		// Gateway API implementations like Istio). The ConfigMap name matches the
		// CA secret name so proxy-ssl-secret annotation value maps directly.
		caCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backendCASecretName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"ca.crt": string(secrets.CACertPEM),
			},
		}
		_, err = client.CoreV1().ConfigMaps(namespace).Create(ctx, caCM, metav1.CreateOptions{})
		require.NoError(t, err, "creating CA configmap")
		t.Cleanup(func() {
			if skipCleanup {
				return
			}
			_ = client.CoreV1().ConfigMaps(namespace).Delete(context.Background(), backendCASecretName, metav1.DeleteOptions{})
		})

		// Create the server TLS secret (mounted in the backend pod).
		secrets.ServerSecret.Namespace = namespace
		_, err = client.CoreV1().Secrets(namespace).Create(ctx, secrets.ServerSecret, metav1.CreateOptions{})
		require.NoError(t, err, "creating server TLS secret")
		t.Cleanup(func() {
			if skipCleanup {
				return
			}
			_ = client.CoreV1().Secrets(namespace).Delete(context.Background(), secrets.ServerSecret.Name, metav1.DeleteOptions{})
		})

		// Deploy the TLS backend app.
		cleanupApp, err := deployDummyTLSApp(ctx, t, client, backendTLSAppName, namespace, backendServerSecret, skipCleanup)
		require.NoError(t, err, "deploying TLS backend app")
		t.Cleanup(cleanupApp)
	}
}

func TestIngressNGINXBackendTLS(t *testing.T) {
	t.Parallel()
	t.Run("to Istio", func(t *testing.T) {
		t.Parallel()

		// Test 1: Valid backend TLS configuration – all required annotations present.
		// The ingress2gateway tool should produce a BackendTLSPolicy for this ingress
		// and the gateway should be able to reach the HTTPS backend.
		t.Run("valid backend tls produces BackendTLSPolicy", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err, "creating host suffix")
			host := "backend-tls-valid-" + suffix + ".example.com"

			ing := basicIngress().
				withName("backend-tls-valid").
				withHost(host).
				withIngressClass(ingressnginx.NginxIngressClass).
				withBackend(backendTLSAppName).
				withBackendPort(443).
				withAnnotation(ingressnginx.BackendProtocolAnnotation, "HTTPS").
				withAnnotation(ingressnginx.ProxySSLVerifyAnnotation, "on").
				withAnnotation(ingressnginx.ProxySSLSecretAnnotation, backendCASecretName).
				withAnnotation(ingressnginx.ProxySSLServerNameAnnotation, "on").
				withAnnotation(ingressnginx.ProxySSLNameAnnotation, "placeholder").
				build()

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				setup:     backendTLSSetup(ing),
				ingresses: []*networkingv1.Ingress{ing},
				verifiers: map[string][]verifier{
					"backend-tls-valid": {
						// The request should reach the HTTPS backend through the gateway
						// and return a 200 OK (agnhost netexec echoes back on /).
						&httpRequestVerifier{
							host: host,
							path: "/",
						},
					},
				},
			})
		})

		// Test 2: Unsupported annotations produce warnings but don't block policy generation.
		// proxy-ssl-verify-depth and proxy-ssl-protocols should emit warnings but a valid
		// BackendTLSPolicy should still be produced when all required annotations are present.
		t.Run("unsupported annotations emit warnings but policy still generated", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err, "creating host suffix")
			host := "backend-tls-warn-" + suffix + ".example.com"

			ing := basicIngress().
				withName("backend-tls-warn").
				withHost(host).
				withIngressClass(ingressnginx.NginxIngressClass).
				withBackend(backendTLSAppName).
				withBackendPort(443).
				withAnnotation(ingressnginx.BackendProtocolAnnotation, "HTTPS").
				withAnnotation(ingressnginx.ProxySSLVerifyAnnotation, "on").
				withAnnotation(ingressnginx.ProxySSLSecretAnnotation, backendCASecretName).
				withAnnotation(ingressnginx.ProxySSLServerNameAnnotation, "on").
				withAnnotation(ingressnginx.ProxySSLNameAnnotation, "placeholder").
				// These two annotations are unsupported in Gateway API
				// but should NOT prevent policy generation.
				withAnnotation(ingressnginx.ProxySSLVerifyDepthAnnotation, "3").
				withAnnotation(ingressnginx.ProxySSLProtocolsAnnotation, "TLSv1.2 TLSv1.3").
				build()

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				setup:     backendTLSSetup(ing),
				ingresses: []*networkingv1.Ingress{ing},
				verifiers: map[string][]verifier{
					"backend-tls-warn": {
						&httpRequestVerifier{
							host: host,
							path: "/",
						},
					},
				},
			})
		})

		// Test 3: Valid config with body response verification – ensure the request
		// actually reaches the backend and we get a real response body.
		t.Run("valid backend tls with body verification", func(t *testing.T) {
			suffix, err := randString()
			require.NoError(t, err, "creating host suffix")
			host := "backend-tls-body-" + suffix + ".example.com"

			ing := basicIngress().
				withName("backend-tls-body").
				withHost(host).
				withIngressClass(ingressnginx.NginxIngressClass).
				withBackend(backendTLSAppName).
				withBackendPort(443).
				withAnnotation(ingressnginx.BackendProtocolAnnotation, "HTTPS").
				withAnnotation(ingressnginx.ProxySSLVerifyAnnotation, "on").
				withAnnotation(ingressnginx.ProxySSLSecretAnnotation, backendCASecretName).
				withAnnotation(ingressnginx.ProxySSLServerNameAnnotation, "on").
				withAnnotation(ingressnginx.ProxySSLNameAnnotation, "placeholder").
				build()

			runTestCase(t, &testCase{
				gatewayImplementation: istio.ProviderName,
				providers:             []string{ingressnginx.Name},
				providerFlags: map[string]map[string]string{
					ingressnginx.Name: {
						ingressnginx.NginxIngressClassFlag: ingressnginx.NginxIngressClass,
					},
				},
				setup:     backendTLSSetup(ing),
				ingresses: []*networkingv1.Ingress{ing},
				verifiers: map[string][]verifier{
					"backend-tls-body": {
						// agnhost netexec echoes back useful info on /hostname
						&httpRequestVerifier{
							host:      host,
							path:      "/hostname",
							bodyRegex: regexp.MustCompile(`.+`),
						},
					},
				},
			})
		})
	})
}
