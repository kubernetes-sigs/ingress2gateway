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

package annotations

import (
	"reflect"
	"testing"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestExtractListenPorts(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		expected   []int32
	}{
		{
			name:       "empty annotation",
			annotation: "",
			expected:   nil,
		},
		{
			name:       "single port",
			annotation: "8080",
			expected:   []int32{8080},
		},
		{
			name:       "multiple ports",
			annotation: "8080,9090,3000",
			expected:   []int32{8080, 9090, 3000},
		},
		{
			name:       "ports with spaces",
			annotation: " 8080 , 9090 , 3000 ",
			expected:   []int32{8080, 9090, 3000},
		},
		{
			name:       "invalid ports filtered",
			annotation: "8080,invalid,9090,0,65536",
			expected:   []int32{8080, 9090},
		},
		{
			name:       "empty parts filtered",
			annotation: "8080,,9090,",
			expected:   []int32{8080, 9090},
		},
		{
			name:       "edge ports",
			annotation: "1,65535",
			expected:   []int32{1, 65535},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractListenPorts(tt.annotation)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCreateListenerName(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		port     int32
		protocol gatewayv1.ProtocolType
		expected string
	}{
		{
			name:     "HTTP listener",
			hostname: "example.com",
			port:     8080,
			protocol: gatewayv1.HTTPProtocolType,
			expected: "example-com-http-8080",
		},
		{
			name:     "HTTPS listener",
			hostname: "api.example.com",
			port:     8443,
			protocol: gatewayv1.HTTPSProtocolType,
			expected: "api-example-com-https-8443",
		},
		{
			name:     "empty hostname",
			hostname: "",
			port:     9090,
			protocol: gatewayv1.HTTPProtocolType,
			expected: "all-hosts-http-9090",
		},
		{
			name:     "wildcard hostname",
			hostname: "*",
			port:     8080,
			protocol: gatewayv1.HTTPProtocolType,
			expected: "all-hosts-http-8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createListenerName(tt.hostname, tt.port, tt.protocol)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCreateListener(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		port     int32
		protocol gatewayv1.ProtocolType
		expected gatewayv1.Listener
	}{
		{
			name:     "HTTP with hostname",
			hostname: "example.com",
			port:     8080,
			protocol: gatewayv1.HTTPProtocolType,
			expected: gatewayv1.Listener{
				Name:     "example-com-http-8080",
				Port:     8080,
				Protocol: gatewayv1.HTTPProtocolType,
				Hostname: (*gatewayv1.Hostname)(ptr.To("example.com")),
			},
		},
		{
			name:     "HTTPS with hostname",
			hostname: "secure.example.com",
			port:     8443,
			protocol: gatewayv1.HTTPSProtocolType,
			expected: gatewayv1.Listener{
				Name:     "secure-example-com-https-8443",
				Port:     8443,
				Protocol: gatewayv1.HTTPSProtocolType,
				Hostname: (*gatewayv1.Hostname)(ptr.To("secure.example.com")),
			},
		},
		{
			name:     "without hostname",
			hostname: "",
			port:     9090,
			protocol: gatewayv1.HTTPProtocolType,
			expected: gatewayv1.Listener{
				Name:     "all-hosts-http-9090",
				Port:     9090,
				Protocol: gatewayv1.HTTPProtocolType,
				Hostname: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createListener(tt.hostname, tt.port, tt.protocol)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestListenPortsFeature(t *testing.T) {
	tests := []struct {
		name              string
		annotations       map[string]string
		expectedListeners int
		expectedHTTPPorts []int32
		expectedSSLPorts  []int32
	}{
		{
			name:              "no custom ports",
			annotations:       map[string]string{},
			expectedListeners: 0,
			expectedHTTPPorts: nil,
			expectedSSLPorts:  nil,
		},
		{
			name: "custom HTTP ports only",
			annotations: map[string]string{
				nginxListenPortsAnnotation: "8080,9090",
			},
			expectedListeners: 3,
			expectedHTTPPorts: []int32{8080, 9090},
			expectedSSLPorts:  []int32{443},
		},
		{
			name: "custom SSL ports only",
			annotations: map[string]string{
				nginxListenPortsSSLAnnotation: "8443,9443",
			},
			expectedListeners: 3,
			expectedHTTPPorts: []int32{80},
			expectedSSLPorts:  []int32{8443, 9443},
		},
		{
			name: "both HTTP and SSL",
			annotations: map[string]string{
				nginxListenPortsAnnotation:    "8080,9090",
				nginxListenPortsSSLAnnotation: "8443,9443",
			},
			expectedListeners: 4,
			expectedHTTPPorts: []int32{8080, 9090},
			expectedSSLPorts:  []int32{8443, 9443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-ingress",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "web-service",
													Port: networkingv1.ServiceBackendPort{Number: 80},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			ir := providerir.ProviderIR{
				Gateways:   make(map[types.NamespacedName]providerir.GatewayContext),
				HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
			}

			errs := ListenPortsFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			if tt.expectedListeners == 0 {
				if len(ir.Gateways) > 0 {
					t.Error("Expected no gateways to be created")
				}
				return
			}

			if len(ir.Gateways) != 1 {
				t.Errorf("Expected 1 gateway, got %d", len(ir.Gateways))
				return
			}

			var gateway gatewayv1.Gateway
			for _, gwContext := range ir.Gateways {
				gateway = gwContext.Gateway
				break
			}

			if len(gateway.Spec.Listeners) != tt.expectedListeners {
				t.Errorf("Expected %d listeners, got %d", tt.expectedListeners, len(gateway.Spec.Listeners))
				return
			}

			httpCount := 0
			for _, listener := range gateway.Spec.Listeners {
				if listener.Protocol == gatewayv1.HTTPProtocolType {
					httpCount++
					found := false
					for _, expectedPort := range tt.expectedHTTPPorts {
						if int32(listener.Port) == expectedPort {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Unexpected HTTP port %d", listener.Port)
					}
				}
			}

			if httpCount != len(tt.expectedHTTPPorts) {
				t.Errorf("Expected %d HTTP listeners, got %d", len(tt.expectedHTTPPorts), httpCount)
			}

			sslCount := 0
			for _, listener := range gateway.Spec.Listeners {
				if listener.Protocol == gatewayv1.HTTPSProtocolType {
					sslCount++
					found := false
					for _, expectedPort := range tt.expectedSSLPorts {
						if int32(listener.Port) == expectedPort {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Unexpected HTTPS port %d", listener.Port)
					}
				}
			}

			if sslCount != len(tt.expectedSSLPorts) {
				t.Errorf("Expected %d HTTPS listeners, got %d", len(tt.expectedSSLPorts), sslCount)
			}

			for _, listener := range gateway.Spec.Listeners {
				if listener.Hostname == nil || string(*listener.Hostname) != "example.com" {
					t.Errorf("Expected hostname 'example.com', got %v", listener.Hostname)
				}
			}
		})
	}
}

func TestListenPortsReplacesDefaultListeners(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				nginxListenPortsAnnotation:    "8080,9090",
				nginxListenPortsSSLAnnotation: "8443",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "web-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Start with IR that has a Gateway with default listeners (simulating what common converter creates)
	gatewayKey := types.NamespacedName{Namespace: "default", Name: "nginx"}
	ir := providerir.ProviderIR{
		Gateways: map[types.NamespacedName]providerir.GatewayContext{
			gatewayKey: {
				Gateway: gatewayv1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nginx",
						Namespace: "default",
					},
					Spec: gatewayv1.GatewaySpec{
						GatewayClassName: "nginx",
						Listeners: []gatewayv1.Listener{
							{
								Name:     "example-com-http",
								Hostname: (*gatewayv1.Hostname)(ptr.To("example.com")),
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
							},
							{
								Name:     "example-com-https",
								Hostname: (*gatewayv1.Hostname)(ptr.To("example.com")),
								Port:     443,
								Protocol: gatewayv1.HTTPSProtocolType,
							},
						},
					},
				},
			},
		},
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}

	// Apply listen-ports feature
	errs := ListenPortsFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	// Verify Gateway was updated
	gateway, exists := ir.Gateways[gatewayKey]
	if !exists {
		t.Error("Gateway should exist")
		return
	}

	// Verify expected custom ports are present
	expectedPorts := map[int32]gatewayv1.ProtocolType{
		8080: gatewayv1.HTTPProtocolType,
		9090: gatewayv1.HTTPProtocolType,
		8443: gatewayv1.HTTPSProtocolType,
	}

	foundPorts := make(map[int32]gatewayv1.ProtocolType)
	for _, listener := range gateway.Gateway.Spec.Listeners {
		foundPorts[int32(listener.Port)] = listener.Protocol

		// Verify hostname is set correctly
		if listener.Hostname == nil || string(*listener.Hostname) != "example.com" {
			t.Errorf("Expected hostname 'example.com', got %v", listener.Hostname)
		}
	}

	for expectedPort, expectedProtocol := range expectedPorts {
		if foundProtocol, exists := foundPorts[expectedPort]; !exists {
			t.Errorf("Expected port %d not found", expectedPort)
		} else if foundProtocol != expectedProtocol {
			t.Errorf("Expected protocol %s for port %d, got %s", expectedProtocol, expectedPort, foundProtocol)
		}
	}
}

func TestListenPortsConflictResolution(t *testing.T) {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-conflict",
			Namespace: "default",
			Annotations: map[string]string{
				nginxListenPortsAnnotation:    "8080,8443,9090",
				nginxListenPortsSSLAnnotation: "8443,9443",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "web-service",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	ir := providerir.ProviderIR{
		Gateways:   make(map[types.NamespacedName]providerir.GatewayContext),
		HTTPRoutes: make(map[types.NamespacedName]providerir.HTTPRouteContext),
	}

	errs := ListenPortsFeature([]networkingv1.Ingress{ingress}, nil, &ir, nil)
	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if len(ir.Gateways) != 1 {
		t.Errorf("Expected 1 gateway, got %d", len(ir.Gateways))
		return
	}

	var gateway gatewayv1.Gateway
	for _, gwContext := range ir.Gateways {
		gateway = gwContext.Gateway
		break
	}

	// Should have 4 listeners: 8080 HTTP, 9090 HTTP, 8443 HTTPS, 9443 HTTPS
	// Note: 8443 should be HTTPS only (not HTTP) due to conflict resolution
	expectedListeners := 4
	if len(gateway.Spec.Listeners) != expectedListeners {
		t.Errorf("Expected %d listeners, got %d", expectedListeners, len(gateway.Spec.Listeners))
		return
	}

	// Verify no port conflicts (same port with different protocols)
	portProtocols := make(map[int32][]gatewayv1.ProtocolType)
	for _, listener := range gateway.Spec.Listeners {
		port := int32(listener.Port)
		portProtocols[port] = append(portProtocols[port], listener.Protocol)
	}

	for port, protocols := range portProtocols {
		if len(protocols) > 1 {
			t.Errorf("Port %d has conflicting protocols: %v", port, protocols)
		}
	}

	// Verify specific expected configurations
	expectedConfigs := map[int32]gatewayv1.ProtocolType{
		8080: gatewayv1.HTTPProtocolType,  // HTTP only
		9090: gatewayv1.HTTPProtocolType,  // HTTP only
		8443: gatewayv1.HTTPSProtocolType, // HTTPS takes precedence over HTTP
		9443: gatewayv1.HTTPSProtocolType, // HTTPS only
	}

	foundConfigs := make(map[int32]gatewayv1.ProtocolType)
	for _, listener := range gateway.Spec.Listeners {
		foundConfigs[int32(listener.Port)] = listener.Protocol
	}

	for expectedPort, expectedProtocol := range expectedConfigs {
		if foundProtocol, exists := foundConfigs[expectedPort]; !exists {
			t.Errorf("Expected port %d not found", expectedPort)
		} else if foundProtocol != expectedProtocol {
			t.Errorf("Expected protocol %s for port %d, got %s", expectedProtocol, expectedPort, foundProtocol)
		}
	}
}
