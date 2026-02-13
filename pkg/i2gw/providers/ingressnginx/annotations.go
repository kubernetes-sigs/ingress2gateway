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
	CanaryByHeader              = "nginx.ingress.kubernetes.io/canary-by-header"
	CanaryByHeaderValue         = "nginx.ingress.kubernetes.io/canary-by-header-value"
	CanaryByHeaderPattern       = "nginx.ingress.kubernetes.io/canary-by-header-pattern"
	CanaryByCookie              = "nginx.ingress.kubernetes.io/canary-by-cookie"

	// Rewrite annotations
	RewriteTargetAnnotation = "nginx.ingress.kubernetes.io/rewrite-target"

	// Header annotations
	XForwardedPrefixAnnotation      = "nginx.ingress.kubernetes.io/x-forwarded-prefix"
	UpstreamVhostAnnotation         = "nginx.ingress.kubernetes.io/upstream-vhost"
	ConnectionProxyHeaderAnnotation = "nginx.ingress.kubernetes.io/connection-proxy-header"
	CustomHeadersAnnotation         = "nginx.ingress.kubernetes.io/custom-headers"

	// Timeout annotations
	ProxyConnectTimeoutAnnotation = "nginx.ingress.kubernetes.io/proxy-connect-timeout"
	ProxySendTimeoutAnnotation    = "nginx.ingress.kubernetes.io/proxy-send-timeout"
	ProxyReadTimeoutAnnotation    = "nginx.ingress.kubernetes.io/proxy-read-timeout"

	// Body Size annotations
	ProxyBodySizeAnnotation        = "nginx.ingress.kubernetes.io/proxy-body-size"
	ClientBodyBufferSizeAnnotation = "nginx.ingress.kubernetes.io/client-body-buffer-size"

	// Regex
	UseRegexAnnotation = "nginx.ingress.kubernetes.io/use-regex"

	// SSL Redirect annotation
	SSLRedirectAnnotation = "nginx.ingress.kubernetes.io/ssl-redirect"

	// CORS annotations
	EnableCorsAnnotation       = "nginx.ingress.kubernetes.io/enable-cors"
	CorsAllowOriginAnnotation  = "nginx.ingress.kubernetes.io/cors-allow-origin"
	CorsAllowHeadersAnnotation = "nginx.ingress.kubernetes.io/cors-allow-headers"
	CorsAllowMethodsAnnotation = "nginx.ingress.kubernetes.io/cors-allow-methods"
	//nolint:gosec // false positive, this is an annotation key
	CorsAllowCredentialsAnnotation = "nginx.ingress.kubernetes.io/cors-allow-credentials"
	CorsExposeHeadersAnnotation    = "nginx.ingress.kubernetes.io/cors-expose-headers"
	CorsMaxAgeAnnotation           = "nginx.ingress.kubernetes.io/cors-max-age"
)
