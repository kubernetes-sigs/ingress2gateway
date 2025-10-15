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
	"fmt"
	"strconv"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

type portConfiguration struct {
	HTTP  []int32
	HTTPS []int32
}

// ListenPortsFeature processes nginx.org/listen-ports and nginx.org/listen-ports-ssl annotations
func ListenPortsFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			httpPorts := extractListenPorts(rule.Ingress.Annotations[nginxListenPortsAnnotation])
			sslPorts := extractListenPorts(rule.Ingress.Annotations[nginxListenPortsSSLAnnotation])

			config := portConfiguration{
				HTTP:  httpPorts,
				HTTPS: sslPorts,
			}
			if len(httpPorts) == 0 {
				config.HTTP = []int32{80} // Default HTTP port
			}

			if len(sslPorts) == 0 {
				config.HTTPS = []int32{443} // Default HTTPS port
			}

			if len(httpPorts) > 0 || len(sslPorts) > 0 {
				errs = append(errs, replaceGatewayPortsWithCustom(*rule.Ingress, config, ir)...)
			}
		}
	}

	return errs
}

// extractListenPorts parses comma-separated port numbers from annotation value
func extractListenPorts(portsAnnotation string) []int32 {
	if portsAnnotation == "" {
		return nil
	}

	var ports []int32
	portStrings := splitAndTrimCommaList(portsAnnotation)

	for _, portStr := range portStrings {
		if port, err := strconv.ParseInt(portStr, 10, 32); err == nil {
			if port > 0 && port <= 65535 {
				ports = append(ports, int32(port))
			}
		}
	}

	return ports
}

// replaceGatewayPortsWithCustom modifies the Gateway to use ONLY the specified custom ports
// This follows NIC behavior where listen-ports annotations REPLACE default ports, not add to them
//
//nolint:unparam // ErrorList return type maintained for consistency
func replaceGatewayPortsWithCustom(ingress networkingv1.Ingress, portConfiguration portConfiguration, ir *intermediate.IR) field.ErrorList {
	var errs field.ErrorList //nolint:unparam // ErrorList return type maintained for consistency

	gatewayClassName := getGatewayClassName(ingress)
	gatewayKey := types.NamespacedName{Namespace: ingress.Namespace, Name: gatewayClassName}

	gatewayContext, exists := ir.Gateways[gatewayKey]
	if !exists {
		gatewayContext = intermediate.GatewayContext{
			Gateway: gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayClassName,
					Namespace: ingress.Namespace,
				},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: gatewayv1.ObjectName(gatewayClassName),
					Listeners:        []gatewayv1.Listener{},
				},
			},
		}
	}

	var filteredListeners []gatewayv1.Listener

	for _, existingListener := range gatewayContext.Gateway.Spec.Listeners {
		keep := true
		for _, rule := range ingress.Spec.Rules {
			hostname := rule.Host
			if existingListener.Hostname != nil && string(*existingListener.Hostname) == hostname {
				if existingListener.Port == 80 && existingListener.Protocol == gatewayv1.HTTPProtocolType {
					keep = false
					break
				}
				if existingListener.Port == 443 && existingListener.Protocol == gatewayv1.HTTPSProtocolType {
					keep = false
					break
				}
			}
		}
		if keep {
			filteredListeners = append(filteredListeners, existingListener)
		}
	}

	for _, rule := range ingress.Spec.Rules {
		hostname := rule.Host

		// Track used ports to avoid conflicts - HTTPS takes precedence over HTTP
		usedPorts := make(map[int32]struct{})

		// Add HTTPS listeners first (they take precedence)
		for _, port := range portConfiguration.HTTPS {
			listener := createListener(hostname, port, gatewayv1.HTTPSProtocolType)

			if len(ingress.Spec.TLS) > 0 {
				listener.TLS = &gatewayv1.GatewayTLSConfig{
					Mode: common.PtrTo(gatewayv1.TLSModeTerminate),
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{
							Name:      gatewayv1.ObjectName(ingress.Spec.TLS[0].SecretName),
							Namespace: (*gatewayv1.Namespace)(&ingress.Namespace),
						},
					},
				}
			}

			filteredListeners = append(filteredListeners, listener)

			usedPorts[port] = struct{}{}
		}

		// Add HTTP listeners only if port not already used by HTTPS
		for _, port := range portConfiguration.HTTP {
			if _, exists := usedPorts[port]; !exists {
				filteredListeners = append(filteredListeners, createListener(hostname, port, gatewayv1.HTTPProtocolType))
			}
		}
	}

	gatewayContext.Gateway.Spec.Listeners = filteredListeners
	ir.Gateways[gatewayKey] = gatewayContext
	return errs
}

// createListener creates a Gateway listener for the given hostname, port, and protocol
func createListener(hostname string, port int32, protocol gatewayv1.ProtocolType) gatewayv1.Listener {
	listenerName := createListenerName(hostname, port, protocol)

	listener := gatewayv1.Listener{
		Name:     gatewayv1.SectionName(listenerName),
		Port:     gatewayv1.PortNumber(port),
		Protocol: protocol,
	}

	if hostname != "" {
		listener.Hostname = (*gatewayv1.Hostname)(&hostname)
	}

	return listener
}

// createListenerName generates a safe listener name from hostname, port, and protocol
func createListenerName(hostname string, port int32, protocol gatewayv1.ProtocolType) string {
	safeName := common.NameFromHost(hostname)
	protocolStr := strings.ToLower(string(protocol))
	return fmt.Sprintf("%s-%s-%d", safeName, protocolStr, port)
}

// getGatewayClassName extracts the gateway class name from ingress
func getGatewayClassName(ingress networkingv1.Ingress) string {
	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName != "" {
		return *ingress.Spec.IngressClassName
	}
	return NginxIngressClass
}
