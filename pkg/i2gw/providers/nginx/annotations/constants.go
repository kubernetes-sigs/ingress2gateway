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

const (
	// NGINX Ingress Controller annotation prefixes
	nginxOrgPrefix = "nginx.org/"

	// Standard annotations that map directly to Gateway API
	nginxRewritesAnnotation        = nginxOrgPrefix + "rewrites"
	nginxRedirectToHTTPSAnnotation = nginxOrgPrefix + "redirect-to-https"

	// Header manipulation annotations
	nginxProxyHideHeadersAnnotation = nginxOrgPrefix + "proxy-hide-headers"
	nginxProxyPassHeadersAnnotation = nginxOrgPrefix + "proxy-pass-headers"
	nginxProxySetHeadersAnnotation  = nginxOrgPrefix + "proxy-set-headers"

	// Port configuration annotations
	nginxListenPortsAnnotation    = nginxOrgPrefix + "listen-ports"
	nginxListenPortsSSLAnnotation = nginxOrgPrefix + "listen-ports-ssl"

	// Backend service annotations
	nginxSSLServicesAnnotation       = nginxOrgPrefix + "ssl-services"
	nginxGRPCServicesAnnotation      = nginxOrgPrefix + "grpc-services"
	nginxWebSocketServicesAnnotation = nginxOrgPrefix + "websocket-services"

	// Path matching annotations
	nginxPathRegexAnnotation = nginxOrgPrefix + "path-regex"

	// Legacy SSL redirect annotation
	legacySSLRedirectAnnotation = "ingress.kubernetes.io/ssl-redirect"

	// HSTS header annotation
	nginxHSTSAnnotation                  = nginxOrgPrefix + "hsts"
	nginxHSTSIncludeSubdomainsAnnotation = nginxOrgPrefix + "hsts-include-subdomains"
	nginxHSTSMaxAgeAnnotation            = nginxOrgPrefix + "hsts-max-age"
)

// NginxIngressClass class name
const (
	NginxIngressClass = "nginx"
)

// Resource kind constants
const (
	BackendTLSPolicyKind     = "BackendTLSPolicy"
	ClientSettingsPolicyKind = "ClientSettingsPolicy"
	ServiceKind              = "Service"
	GRPCRouteKind            = "GRPCRoute"
)

// Resource group constants
const (
	CoreGroup = "" // Core Kubernetes resources (Service, etc.)
)
