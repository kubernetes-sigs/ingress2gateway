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

package crds

import (
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx/common"
	nginxv1 "github.com/nginx/kubernetes-ingress/pkg/apis/configuration/v1"
)

const (
	// DefaultGatewayName is the default name for gateways created from VirtualServers
	DefaultGatewayName = "nginx"

	namespaceGatewayHTTPPort  = 80
	namespaceGatewayHTTPSPort = 443
)

type listenerKey struct {
	Port     int
	Protocol gatewayv1.ProtocolType
	Hostname string
}

type gatewayListenerKey struct {
	gatewayName  string
	listenerName string
}

// NamespaceGatewayFactory creates shared Gateway resources for VirtualServers and TransportServers in a namespace
type NamespaceGatewayFactory struct {
	namespace        string
	virtualServers   []nginxv1.VirtualServer
	transportServers []nginxv1.TransportServer
	notificationList *[]notifications.Notification
	listenerMap      map[string]gatewayv1.Listener
}

// NewNamespaceGatewayFactory creates a new factory for namespace-scoped Gateway creation
func NewNamespaceGatewayFactory(namespace string, virtualServers []nginxv1.VirtualServer, transportServers []nginxv1.TransportServer, notifs *[]notifications.Notification, listenerMap map[string]gatewayv1.Listener) *NamespaceGatewayFactory {
	return &NamespaceGatewayFactory{
		namespace:        namespace,
		virtualServers:   virtualServers,
		transportServers: transportServers,
		notificationList: notifs,
		listenerMap:      listenerMap,
	}
}

// CreateNamespaceGateway creates a single Gateway for all VirtualServers and TransportServers in the namespace
func (f *NamespaceGatewayFactory) CreateNamespaceGateway() (map[types.NamespacedName]intermediate.GatewayContext, map[string][]gatewayListenerKey) {
	gatewayName := DefaultGatewayName
	gatewayKey := types.NamespacedName{
		Namespace: f.namespace,
		Name:      gatewayName,
	}

	gateways := make(map[types.NamespacedName]intermediate.GatewayContext)

	// Create all listeners for the single gateway
	listeners, virtualServerMap := f.createListeners(gatewayName)

	gateways[gatewayKey] = intermediate.GatewayContext{
		Gateway: gatewayv1.Gateway{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gatewayv1.GroupVersion.String(),
				Kind:       "Gateway",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      gatewayName,
				Namespace: f.namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "ingress2gateway",
					"ingress2gateway.io/source":    "nginx-virtualserver",
				},
			},
			Spec: gatewayv1.GatewaySpec{
				GatewayClassName: "nginx",
				Listeners:        listeners,
			},
		},
		ProviderSpecificIR: intermediate.ProviderSpecificGatewayIR{
			Nginx: &intermediate.NginxGatewayIR{},
		},
	}

	return gateways, virtualServerMap
}

// createListeners creates HTTP and HTTPS listeners for the Gateway
func (f *NamespaceGatewayFactory) createListeners(gatewayName string) ([]gatewayv1.Listener, map[string][]gatewayListenerKey) {
	uniqueListeners := make(map[listenerKey]gatewayv1.Listener)
	virtualServerMap := make(map[string][]gatewayListenerKey)

	for _, vs := range f.virtualServers {
		httpPort, httpsPort := f.getListenerPorts(vs)
		hostPtr := &vs.Spec.Host
		if vs.Spec.Host == "" {
			hostPtr = nil
		}

		// HTTP
		if httpPort > 0 {
			key := listenerKey{
				Port:     httpPort,
				Protocol: gatewayv1.HTTPProtocolType,
				Hostname: vs.Spec.Host,
			}

			redirect := false
			if vs.Spec.TLS != nil && vs.Spec.TLS.Redirect != nil && vs.Spec.TLS.Redirect.Enable {
				redirect = true
			}

			if _, exists := uniqueListeners[key]; !exists {
				listenerName := fmt.Sprintf("http-%d-%s", httpPort, sanitizeHostname(vs.Spec.Host))
				uniqueListeners[key] = gatewayv1.Listener{
					Name:     gatewayv1.SectionName(listenerName),
					Port:     gatewayv1.PortNumber(httpPort),
					Protocol: gatewayv1.HTTPProtocolType,
					Hostname: (*gatewayv1.Hostname)(hostPtr),
				}
			}

			if !redirect {
				virtualServerMap[vs.Name] = append(virtualServerMap[vs.Name], gatewayListenerKey{
					gatewayName:  gatewayName,
					listenerName: string(uniqueListeners[key].Name),
				})
			}
		}

		// HTTPS
		if httpsPort > 0 {
			secret := ""
			if vs.Spec.TLS != nil && vs.Spec.TLS.Secret != "" {
				secret = vs.Spec.TLS.Secret
			}

			key := listenerKey{
				Port:     httpsPort,
				Protocol: gatewayv1.HTTPSProtocolType,
				Hostname: vs.Spec.Host,
			}

			if _, exists := uniqueListeners[key]; !exists {
				listenerName := fmt.Sprintf("https-%d-%s-%s", httpsPort, sanitizeHostname(vs.Spec.Host), sanitizeSecret(secret))
				uniqueListeners[key] = gatewayv1.Listener{
					Name:     gatewayv1.SectionName(listenerName),
					Port:     gatewayv1.PortNumber(httpsPort),
					Protocol: gatewayv1.HTTPSProtocolType,
					Hostname: (*gatewayv1.Hostname)(hostPtr),
					TLS: &gatewayv1.GatewayTLSConfig{
						Mode: Ptr(gatewayv1.TLSModeTerminate),
						CertificateRefs: []gatewayv1.SecretObjectReference{
							{
								Group: Ptr(gatewayv1.Group(common.CoreGroup)),
								Kind:  Ptr(gatewayv1.Kind(common.SecretKind)),
								Name:  gatewayv1.ObjectName(secret),
							},
						},
					},
				}
			}

			virtualServerMap[vs.Name] = append(virtualServerMap[vs.Name], gatewayListenerKey{
				gatewayName:  gatewayName,
				listenerName: string(uniqueListeners[key].Name),
			})
		}
	}

	// Process TransportServers to create TCP/TLS/UDP listeners
	transportServerMap := make(map[string][]gatewayListenerKey)
	for _, ts := range f.transportServers {
		port := f.getTransportServerPort(ts)
		if port == nil {
			continue
		}
		protocol := f.getTransportServerProtocol(ts)
		if protocol == nil {
			continue
		}

		key := listenerKey{
			Port:     *port,
			Protocol: *protocol,
			Hostname: ts.Spec.Host, // May be empty for non-SNI protocols
		}

		listenerName := f.generateTransportListenerName(ts, *port, *protocol)

		if _, exists := uniqueListeners[key]; !exists {
			listener := gatewayv1.Listener{
				Name:     gatewayv1.SectionName(listenerName),
				Port:     gatewayv1.PortNumber(*port),
				Protocol: *protocol,
			}

			// Add hostname for TLS routes that use SNI
			if ts.Spec.Host != "" && *protocol == gatewayv1.TLSProtocolType {
				hostname := gatewayv1.Hostname(ts.Spec.Host)
				listener.Hostname = &hostname
			}

			// Add TLS configuration for TLS listeners
			if *protocol == gatewayv1.TLSProtocolType {
				if ts.Spec.TLS != nil && ts.Spec.TLS.Secret != "" {
					// TLS termination
					listener.TLS = &gatewayv1.GatewayTLSConfig{
						Mode: Ptr(gatewayv1.TLSModeTerminate),
						CertificateRefs: []gatewayv1.SecretObjectReference{
							{
								Group: Ptr(gatewayv1.Group(common.CoreGroup)),
								Kind:  Ptr(gatewayv1.Kind(common.SecretKind)),
								Name:  gatewayv1.ObjectName(ts.Spec.TLS.Secret),
							},
						},
					}
				} else {
					// TLS passthrough (default for TLS_PASSTHROUGH protocol)
					listener.TLS = &gatewayv1.GatewayTLSConfig{
						Mode: Ptr(gatewayv1.TLSModePassthrough),
					}
				}
			}

			uniqueListeners[key] = listener
		}

		// Map TransportServer to listener
		transportServerMap[ts.Name] = append(transportServerMap[ts.Name], gatewayListenerKey{
			gatewayName:  gatewayName,
			listenerName: listenerName,
		})
	}

	// Merge virtualServerMap and transportServerMap
	for tsName, listeners := range transportServerMap {
		virtualServerMap[tsName] = listeners
	}

	// Sort virtualServerMap entries for deterministic parent references
	for vsName, listenerKeys := range virtualServerMap {
		sort.Slice(listenerKeys, func(i, j int) bool {
			if listenerKeys[i].gatewayName != listenerKeys[j].gatewayName {
				return listenerKeys[i].gatewayName < listenerKeys[j].gatewayName
			}
			return listenerKeys[i].listenerName < listenerKeys[j].listenerName
		})
		virtualServerMap[vsName] = listenerKeys
	}

	// Convert map to slice and sort for deterministic order
	var listeners []gatewayv1.Listener
	for _, listener := range uniqueListeners {
		listeners = append(listeners, listener)
	}

	// Sort listeners by name for deterministic ordering
	sort.Slice(listeners, func(i, j int) bool {
		return string(listeners[i].Name) < string(listeners[j].Name)
	})

	return listeners, virtualServerMap
}

// getListenerPorts determines which ports/protocols a VirtualServer needs
func (f *NamespaceGatewayFactory) getListenerPorts(vs nginxv1.VirtualServer) (httpPort, httpsPort int) {
	// Check if VirtualServer specifies custom listeners
	if vs.Spec.Listener != nil {
		// Use custom listeners from GlobalConfiguration
		if vs.Spec.Listener.HTTP != "" {
			if listener, found := f.listenerMap[vs.Spec.Listener.HTTP]; found {
				if listener.Protocol == gatewayv1.HTTPProtocolType {
					httpPort = int(listener.Port)
				}
			}
		}
		if vs.Spec.Listener.HTTPS != "" {
			if listener, found := f.listenerMap[vs.Spec.Listener.HTTPS]; found {
				if listener.Protocol == gatewayv1.HTTPSProtocolType {
					httpsPort = int(listener.Port)
				}
			}
		}
	} else {
		// Use default ports
		httpPort = namespaceGatewayHTTPPort
		if vs.Spec.TLS != nil {
			httpsPort = namespaceGatewayHTTPSPort
		}
	}
	return httpPort, httpsPort
}

// getTransportServerPort determines the port for a TransportServer from GlobalConfiguration or defaults
func (f *NamespaceGatewayFactory) getTransportServerPort(ts nginxv1.TransportServer) *int {
	listenerName := ts.Spec.Listener.Name
	if listener, exists := f.listenerMap[listenerName]; exists {
		return Ptr(int(listener.Port))
	}
	return nil
}

// getTransportServerProtocol maps TransportServer protocol to Gateway API protocol
func (f *NamespaceGatewayFactory) getTransportServerProtocol(ts nginxv1.TransportServer) *gatewayv1.ProtocolType {
	switch ts.Spec.Listener.Protocol {
	case "TCP":
		return Ptr(gatewayv1.TCPProtocolType)
	case "UDP":
		return Ptr(gatewayv1.UDPProtocolType)
	case "TLS_PASSTHROUGH":
		return Ptr(gatewayv1.TLSProtocolType)
	default:
		return nil
	}
}

// generateTransportListenerName creates a listener name for TransportServer
func (f *NamespaceGatewayFactory) generateTransportListenerName(ts nginxv1.TransportServer, port int, protocol gatewayv1.ProtocolType) string {
	protocolStr := strings.ToLower(string(protocol))

	if ts.Spec.Host != "" {
		hostname := sanitizeHostname(ts.Spec.Host)
		return fmt.Sprintf("%s-%d-%s", protocolStr, port, hostname)
	}

	return fmt.Sprintf("%s-%d", protocolStr, port)
}

// addNotification adds a notification to the notification list
func (f *NamespaceGatewayFactory) addNotification(messageType notifications.MessageType, message string) {
	// Use the first VirtualServer as the source object for namespace-level notifications
	var sourceObject *nginxv1.VirtualServer
	if len(f.virtualServers) > 0 {
		sourceObject = &f.virtualServers[0]
	}

	addNotification(f.notificationList, messageType, message, sourceObject)
}

// sanitizeHostname replaces special characters for use in sectionName
func sanitizeHostname(host string) string {
	if host == "" {
		return "catchall"
	}
	host = strings.ToLower(host)
	host = strings.ReplaceAll(host, ".", "-")
	host = strings.ReplaceAll(host, "*", "wildcard")
	host = strings.ReplaceAll(host, ":", "-")
	host = strings.ReplaceAll(host, "_", "-")
	host = strings.Trim(host, "-")
	if host == "" {
		return "catchall"
	}
	if len(host) > 30 { // Keep it shorter for section names
		host = host[:30]
	}
	return host
}

// sanitizeSecret replaces special characters in secret names for use in sectionName
func sanitizeSecret(secret string) string {
	if secret == "" {
		return "nosecret"
	}
	secret = strings.ToLower(secret)
	secret = strings.ReplaceAll(secret, ".", "-")
	secret = strings.ReplaceAll(secret, "_", "-")
	secret = strings.Trim(secret, "-")
	if secret == "" {
		return "nosecret"
	}
	if len(secret) > 20 { // Keep it shorter for section names
		secret = secret[:20]
	}
	return secret
}
