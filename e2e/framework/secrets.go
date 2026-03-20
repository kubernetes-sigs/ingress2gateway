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

package framework

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TLSTestSecret holds a TLS secret and its CA certificate for testing.
type TLSTestSecret struct {
	Secret *corev1.Secret
	CACert []byte
}

// BackendTLSSecrets holds the secrets needed for backend TLS authentication testing.
type BackendTLSSecrets struct {
	// ServerSecret contains the TLS cert+key for the HTTPS backend pod.
	ServerSecret *corev1.Secret
	// CASecret contains the CA certificate used to verify the backend, referenced by the
	// proxy-ssl-secret annotation and later by BackendTLSPolicy.
	CASecret  *corev1.Secret // ca.crt for the proxy-ssl-secret / BackendTLSPolicy
	CACertPEM []byte
}

// GenerateSelfSignedTLSSecret creates a self-signed TLS secret for testing.
func GenerateSelfSignedTLSSecret(name, commonName string, hosts []string) (*TLSTestSecret, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
		DNSNames:              hosts,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("creating certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshaling private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return &TLSTestSecret{
		Secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
		CACert: certPEM,
	}, nil
}

// Creates Kubernetes Secret resources and returns a cleanup function.
func createSecrets(ctx context.Context, l Logger, client *kubernetes.Clientset, ns string, secrets []*corev1.Secret, skipCleanup bool) (func(), error) {
	for _, secret := range secrets {
		if secret.Namespace == "" {
			secret.Namespace = ns
		}

		y, err := toYAML(secret)
		if err != nil {
			return nil, fmt.Errorf("converting secret to YAML: %w", err)
		}

		l.Logf("Creating secret:\n%s", y)

		_, err = client.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of secrets")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, secret := range secrets {
			namespace := secret.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting secret %s/%s", namespace, secret.Name)
			err := client.CoreV1().Secrets(namespace).Delete(cleanupCtx, secret.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting secret %s: %v", secret.Name, err)
			}
		}
	}, nil
}

const (
	BackendServerSecretName = "tls-backend-server-cert" //nolint:gosec // Not a credential, just a resource name.
	BackendCASecretName     = "tls-backend-ca"          //nolint:gosec // Not a credential, just a resource name.
)

// GenerateBackendTLSSecrets creates a self-signed CA and a server certificate
// signed by that CA, returning them as Kubernetes TLS and Opaque secrets.
func GenerateBackendTLSSecrets(serverSecretName, caSecretName, namespace, serverHostname string) (*BackendTLSSecrets, error) {
	// Generate CA key and self-signed CA cert.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating CA key: %w", err)
	}
	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating CA serial: %w", err)
	}
	caTemplate := x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: "backend-tls-ca"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("creating CA certificate: %w", err)
	}
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caKeyBytes, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling CA key: %w", err)
	}
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caKeyBytes})
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	// Generate server key and cert signed by the CA.
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating server key: %w", err)
	}
	serverSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating server serial: %w", err)
	}
	serverTemplate := x509.Certificate{
		SerialNumber: serverSerial,
		Subject:      pkix.Name{CommonName: serverHostname},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{serverHostname},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("creating server certificate: %w", err)
	}

	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	serverKeyBytes, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling server key: %w", err)
	}
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyBytes})

	return &BackendTLSSecrets{
		ServerSecret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serverSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       serverCertPEM,
				corev1.TLSPrivateKeyKey: serverKeyPEM,
			},
		},
		CASecret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"ca.crt":  caCertPEM, // CA cert to verify the backend server
				"tls.crt": caCertPEM, // client cert the proxy presents to the backend
				"tls.key": caKeyPEM,  // client key matching tls.crt
			},
		},
		CACertPEM: caCertPEM,
	}, nil
}
