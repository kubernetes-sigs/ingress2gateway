/*
Copyright 2023 The Kubernetes Authors.

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

package istio

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	istiov1beta1 "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type converter struct {
	// gw -> namespace -> hosts; stores hosts allowed by each Gateway
	gwAllowedHosts map[types.NamespacedName]map[string]sets.Set[string]
}

func newConverter() converter {
	return converter{
		gwAllowedHosts: make(map[types.NamespacedName]map[string]sets.Set[string]),
	}
}

func (c *converter) convert(storage storage) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources := i2gw.GatewayResources{
		Gateways:        make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes:      make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		TLSRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		ReferenceGrants: make(map[types.NamespacedName]gatewayv1alpha2.ReferenceGrant),
	}

	rootPath := field.NewPath(ProviderName)

	for _, istioGateway := range storage.Gateways {
		gw := c.convertGateway(istioGateway, rootPath)

		gatewayResources.Gateways[types.NamespacedName{
			Namespace: gw.Namespace,
			Name:      gw.Name,
		}] = *gw
	}

	return gatewayResources, nil
}

func (c *converter) convertGateway(gw *istioclientv1beta1.Gateway, fieldPath *field.Path) *gatewayv1.Gateway {
	apiVersion, kind := common.GatewayGVK.ToAPIVersionAndKind()
	gwPath := fieldPath.Child("Gateway").Key(gw.Name)

	var listeners []gatewayv1.Listener

	// namespace -> hosts
	gwAllowedHosts := make(map[string]sets.Set[string])

	for i, server := range gw.Spec.GetServers() {
		serverName := fmt.Sprintf("%v", i)
		if server.GetName() != "" {
			serverName = server.GetName()
		}
		serverFieldPath := gwPath.Child("Server").Key(serverName)

		serverPort := server.GetPort()
		if serverPort == nil {
			klog.Error(field.Invalid(serverFieldPath, nil, "port is nil"))
			continue
		}

		portFieldPath := serverFieldPath.Child("Port")

		if serverPort.GetName() != "" {
			klog.Infof("ignoring field: %v", portFieldPath.Child("Name"))
		}

		var protocol gatewayv1.ProtocolType
		switch serverPortProtocol := serverPort.GetProtocol(); serverPortProtocol {
		case "HTTP", "HTTPS", "TCP", "TLS":
			protocol = gatewayv1.ProtocolType(serverPortProtocol)
		case "HTTP2", "GRPC":
			if server.GetTls() != nil {
				protocol = gatewayv1.HTTPSProtocolType
			} else {
				protocol = gatewayv1.HTTPProtocolType
			}
		case "MONGO":
			protocol = gatewayv1.TCPProtocolType
		default:
			klog.Infof("unknown field value, ignoring: %v", portFieldPath.Child("Protocol").Key(serverPortProtocol))
			continue
		}

		var tlsMode gatewayv1.TLSModeType
		if serverTLS := server.GetTls(); serverTLS != nil {
			tlsFieldPath := serverFieldPath.Child("TLS")

			switch serverTLSMode := serverTLS.GetMode(); serverTLSMode {
			case istiov1beta1.ServerTLSSettings_PASSTHROUGH, istiov1beta1.ServerTLSSettings_AUTO_PASSTHROUGH:
				tlsMode = gatewayv1.TLSModePassthrough
			case istiov1beta1.ServerTLSSettings_SIMPLE, istiov1beta1.ServerTLSSettings_MUTUAL:
				tlsMode = gatewayv1.TLSModeTerminate
			case istiov1beta1.ServerTLSSettings_ISTIO_MUTUAL, istiov1beta1.ServerTLSSettings_OPTIONAL_MUTUAL:
				klog.Infof("unsupported field value, ignoring: %v", tlsFieldPath.Child("Mode").Key(serverTLSMode.String()))
			default:
				klog.Infof("unknown field value, ignoring: %v", tlsFieldPath.Child("Mode").Key(serverTLSMode.String()))
			}

			if serverTLS.GetHttpsRedirect() {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("HttpsRedirect"))
			}
			if serverTLS.GetServerCertificate() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("ServerCertificate"))
			}
			if serverTLS.GetPrivateKey() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("PrivateKey"))
			}
			if serverTLS.GetCaCertificates() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("CaCertificates"))
			}
			if len(serverTLS.GetSubjectAltNames()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("SubjectAltNames"))
			}
			if serverTLS.GetCredentialName() != "" {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("CredentialName"))
			}
			if len(serverTLS.GetVerifyCertificateSpki()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("VerifyCertificateSpki"))
			}
			if len(serverTLS.GetVerifyCertificateHash()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("VerifyCertificateHash"))
			}
			if serverTLS.GetMinProtocolVersion() != 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("MinProtocolVersion"))
			}
			if serverTLS.GetMaxProtocolVersion() != 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("MaxProtocolVersion"))
			}
			if len(serverTLS.GetCipherSuites()) > 0 {
				klog.Infof("ignoring field: %v", tlsFieldPath.Child("CipherSuites"))
			}
		}

		if server.GetBind() != "" {
			klog.Infof("ignoring field: %v", serverFieldPath.Child("Bind").Key(server.GetBind()))
		}

		for _, host := range server.GetHosts() {
			gwListener := gatewayv1.Listener{
				Port:     gatewayv1.PortNumber(server.GetPort().GetNumber()),
				Protocol: protocol,
			}
			if tlsMode != "" {
				gwListener.TLS = &gatewayv1.GatewayTLSConfig{
					Mode: &tlsMode,
				}
			}

			namespace, dnsName, ok := strings.Cut(host, "/")
			if !ok {
				// The default, if no `namespace/` is specified, is `*/`, that is, select services from any namespace.
				namespace, dnsName = "*", host
			}

			if dnsName != "*" && !strings.Contains(dnsName, ".") {
				klog.Warningf("ignoring non-FQDN formatted host: %v", serverFieldPath.Child("Hosts").Key(dnsName))
				continue
			}

			// if dnsName == "*", then gwListener is empty which matches all hostnames for the listener
			if dnsName != "*" {
				gwListener.Hostname = common.PtrTo[gatewayv1.Hostname](gatewayv1.Hostname(dnsName))
			}

			if _, ok := gwAllowedHosts[namespace]; !ok {
				gwAllowedHosts[namespace] = sets.New[string]()
			}
			gwAllowedHosts[namespace].Insert(dnsName)

			gwListenerName := strings.ToLower(fmt.Sprintf("%v-protocol-%v-ns-%v", protocol, namespace, dnsName))
			if namespace == "." {
				gwListenerName = strings.ToLower(fmt.Sprintf("%v-protocol-dot-ns-%v", protocol, dnsName))
			}
			gwListenerName = strings.Replace(gwListenerName, "*", "wildcard", -1)

			// listener name should match RFC 1123 subdomain requirement: lowercase alphanumeric characters, '-' or '.', and must start and end with a lowercase alphanumeric character
			gwListener.Name = gatewayv1.SectionName(gwListenerName)

			listeners = append(listeners, gwListener)
		}
	}

	c.gwAllowedHosts[types.NamespacedName{
		Namespace: gw.Namespace,
		Name:      gw.Name,
	}] = gwAllowedHosts

	return &gatewayv1.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       gw.Namespace,
			Name:            gw.Name,
			Labels:          gw.Labels,
			Annotations:     gw.Annotations,
			OwnerReferences: gw.OwnerReferences,
			Finalizers:      gw.Finalizers,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: K8SGatewayClassName,
			Listeners:        listeners,
		},
	}
}
