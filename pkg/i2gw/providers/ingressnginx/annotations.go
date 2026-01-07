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

package ingressnginx

const (
	// Canary annotations
	CanaryAnnotation            = "nginx.ingress.kubernetes.io/canary"
	CanaryWeightAnnotation      = "nginx.ingress.kubernetes.io/canary-weight"
	CanaryWeightTotalAnnotation = "nginx.ingress.kubernetes.io/canary-weight-total"

	// Rewrite annotations
	RewriteTargetAnnotation = "nginx.ingress.kubernetes.io/rewrite-target"

	// Header annotations
	XForwardedPrefixAnnotation      = "nginx.ingress.kubernetes.io/x-forwarded-prefix"
	UpstreamVhostAnnotation         = "nginx.ingress.kubernetes.io/upstream-vhost"
	ConnectionProxyHeaderAnnotation = "nginx.ingress.kubernetes.io/connection-proxy-header"
	CustomHeadersAnnotation         = "nginx.ingress.kubernetes.io/custom-headers"

	// R2gex
	UseRegexAnnotation = "nginx.ingress.kubernetes.io/use-regex"

	// SSL Redirect annotation
	SSLRedirectAnnotation = "nginx.ingress.kubernetes.io/ssl-redirect"

	// Timeout annotations
	ProxyConnectTimeoutAnnotation      = "nginx.ingress.kubernetes.io/proxy-connect-timeout"
	ProxySendTimeoutAnnotation         = "nginx.ingress.kubernetes.io/proxy-send-timeout"
	ProxyReadTimeoutAnnotation         = "nginx.ingress.kubernetes.io/proxy-read-timeout"
	ProxyNextUpstreamAnnotation        = "nginx.ingress.kubernetes.io/proxy-next-upstream"
	ProxyNextUpstreamTimeoutAnnotation = "nginx.ingress.kubernetes.io/proxy-next-upstream-timeout"
	ProxyNextUpstreamTriesAnnotation   = "nginx.ingress.kubernetes.io/proxy-next-upstream-tries"

	// Proxy body size annotation (not a timeout, but related to proxy configuration)
	ProxyBodySizeAnnotation = "nginx.ingress.kubernetes.io/proxy-body-size"
)
