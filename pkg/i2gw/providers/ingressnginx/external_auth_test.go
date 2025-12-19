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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_parseAuthUrl(t *testing.T) {
	testCases := []struct {
		name             string
		authUrl          string
		ingressNamespace string
		expectedAuthInfo *authServiceInfo
		expectError      bool
		errorContains    string
	}{
		{
			name:    "valid FQDN with port and path",
			authUrl: "http://auth-service.auth-ns.svc.cluster.local:9000/validate",
			expectedAuthInfo: &authServiceInfo{
				serviceName: "auth-service",
				namespace:   "auth-ns",
				port:        9000,
				path:        "/validate",
			},
			expectError: false,
		},
		{
			name:    "FQDN without port defaults to 80",
			authUrl: "http://auth-service.auth-ns.svc.cluster.local/validate",
			expectedAuthInfo: &authServiceInfo{
				serviceName: "auth-service",
				namespace:   "auth-ns",
				port:        80,
				path:        "/validate",
			},
			expectError: false,
		},
		{
			name:    "HTTPS FQDN without port defaults to 443",
			authUrl: "https://auth-service.auth-ns.svc.cluster.local/validate",
			expectedAuthInfo: &authServiceInfo{
				serviceName: "auth-service",
				namespace:   "auth-ns",
				port:        443,
				path:        "/validate",
			},
			expectError: false,
		},
		{
			name:    "FQDN with empty path",
			authUrl: "http://auth-service.auth-ns.svc.cluster.local:9000",
			expectedAuthInfo: &authServiceInfo{
				serviceName: "auth-service",
				namespace:   "auth-ns",
				port:        9000,
				path:        "",
			},
			expectError: false,
		},
		{
			name:    "FQDN with complex path",
			authUrl: "http://oauth2-proxy.auth-system.svc.cluster.local:4180/oauth2/auth",
			expectedAuthInfo: &authServiceInfo{
				serviceName: "oauth2-proxy",
				namespace:   "auth-system",
				port:        4180,
				path:        "/oauth2/auth",
			},
			expectError: false,
		},
		{
			name:          "URL without .svc.cluster.local suffix is rejected",
			authUrl:       "http://auth-service.auth-ns:9000/validate",
			expectError:   true,
			errorContains: "must use Kubernetes service FQDN with .svc.cluster.local suffix",
		},
		{
			name:          "external URL is rejected",
			authUrl:       "https://auth.example.com/validate",
			expectError:   true,
			errorContains: "must use Kubernetes service FQDN with .svc.cluster.local suffix",
		},
		{
			name:          "invalid URL",
			authUrl:       "not a url",
			expectError:   true,
			errorContains: "auth url must contain a hostname",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						extAuthUrlAnnotation: tc.authUrl,
					},
				},
			}

			authInfo, err := parseAuthUrl(ingress)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Fatalf("expected error containing %q, got %q", tc.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedAuthInfo, authInfo, cmp.AllowUnexported(authServiceInfo{})); diff != "" {
				t.Fatalf("parseAuthUrl() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_parseResponseHeaders(t *testing.T) {
	testCases := []struct {
		name            string
		headers         string
		expectedHeaders []string
	}{
		{
			name:            "single header",
			headers:         "X-Auth-User",
			expectedHeaders: []string{"X-Auth-User"},
		},
		{
			name:            "multiple headers",
			headers:         "X-Auth-User,X-Auth-Email,X-Auth-Name",
			expectedHeaders: []string{"X-Auth-User", "X-Auth-Email", "X-Auth-Name"},
		},
		{
			name:            "headers with spaces",
			headers:         "X-Auth-User, X-Auth-Email , X-Auth-Name",
			expectedHeaders: []string{"X-Auth-User", "X-Auth-Email", "X-Auth-Name"},
		},
		{
			name:            "empty string",
			headers:         "",
			expectedHeaders: nil,
		},
		{
			name:            "single header with trailing comma",
			headers:         "X-Auth-User,",
			expectedHeaders: []string{"X-Auth-User"},
		},
		{
			name:            "headers with extra commas",
			headers:         "X-Auth-User,,X-Auth-Email",
			expectedHeaders: []string{"X-Auth-User", "X-Auth-Email"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
					Annotations: map[string]string{
						extAuthResponseHeadersAnnotation: tc.headers,
					},
				},
			}

			result := parseResponseHeaders(ingress)

			if diff := cmp.Diff(tc.expectedHeaders, result); diff != "" {
				t.Fatalf("parseResponseHeaders() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
