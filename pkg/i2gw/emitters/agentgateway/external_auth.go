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

package agentgateway

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	"github.com/agentgateway/agentgateway/controller/api/v1alpha1/shared"
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"

	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyExtAuthPolicy projects the ExtAuth IR policy into an AgentgatewayPolicy.
//
// Semantics:
//   - agentgateway supports extAuth natively under AgentgatewayPolicy.spec.traffic.extAuth.
//   - We parse nginx.ingress.kubernetes.io/auth-url and map it to:
//     spec.traffic.extAuth.backendRef -> Service reference
//     spec.traffic.extAuth.http.path  -> constant CEL string literal (when not "/")
//     spec.traffic.extAuth.http.allowedResponseHeaders -> pol.ExtAuth.ResponseHeaders (when set)
//
// Notes:
//   - External auth URLs (non-.svc) are skipped by this mapping.
//   - This mapping assumes HTTP-mode external auth (Ingress NGINX auth-url semantics).
func applyExtAuthPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.ExtAuth == nil || pol.ExtAuth.AuthURL == "" {
		return false
	}

	parsed, err := parseAuthURL(pol.ExtAuth.AuthURL, namespace)
	if err != nil {
		// Invalid URL, skip it.
		return false
	}
	if parsed.external {
		// Skip external URLs; this mapping only references in-cluster Services.
		return false
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Traffic == nil {
		agp.Spec.Traffic = &agentgatewayv1alpha1.Traffic{}
	}

	extAuth := &agentgatewayv1alpha1.ExtAuth{
		BackendRef: gatewayv1.BackendObjectReference{
			Name:      gatewayv1.ObjectName(parsed.service),
			Namespace: ptr.To(gatewayv1.Namespace(parsed.namespace)),
			Port:      ptr.To(gatewayv1.PortNumber(parsed.port)),
		},
		HTTP: &agentgatewayv1alpha1.AgentExtAuthHTTP{},
	}

	// Only set a constant Path expression if it's not the default "/" to avoid redirect/sign-in edge cases.
	if parsed.path != "" && parsed.path != "/" {
		// CEL string literal for a constant path.
		p := shared.CELExpression(fmt.Sprintf("%q", parsed.path))
		extAuth.HTTP.Path = &p
	}

	// Pass selected response headers from the auth service to the upstream backend request.
	if len(pol.ExtAuth.ResponseHeaders) > 0 {
		allowed := make([]agentgatewayv1alpha1.ShortString, 0, len(pol.ExtAuth.ResponseHeaders))
		for _, h := range pol.ExtAuth.ResponseHeaders {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			allowed = append(allowed, agentgatewayv1alpha1.ShortString(h))
		}
		if len(allowed) > 0 {
			extAuth.HTTP.AllowedResponseHeaders = allowed
		}
	}

	agp.Spec.Traffic.ExtAuth = extAuth
	ap[ingressName] = agp
	return true
}

// parsedAuthURL contains the fields you can use to build a BackendObjectReference.
type parsedAuthURL struct {
	service   string
	namespace string
	port      int32
	path      string
	external  bool // true if host is not a Kubernetes service
}

// parseAuthURL parses an nginx.ingress.kubernetes.io/auth-url value into a parsedAuthURL.
//
// Expected form (cluster-local service):
//
//	http(s)://<svc>.<ns>.svc[:port]/path
//
// For non-.svc hosts, this returns external=true (unsupported by this emitter mapping).
func parseAuthURL(raw string, ingressNS string) (*parsedAuthURL, error) {
	if raw == "" {
		return nil, fmt.Errorf("auth-url is empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid auth-url: %w", err)
	}

	// Default path
	path := u.Path
	if path == "" {
		path = "/"
	}

	host := u.Host

	// Split host and port
	var hostname, portStr string
	if h, p, err := net.SplitHostPort(host); err == nil {
		hostname = h
		portStr = p
	} else {
		hostname = host
	}

	// Detect external hostname (not a Kubernetes service)
	if !strings.Contains(hostname, ".svc") {
		return &parsedAuthURL{
			external: true,
			path:     path,
		}, nil
	}

	// Normalize cluster-local suffixes
	hostname = strings.TrimSuffix(hostname, ".cluster.local")
	hostname = strings.TrimSuffix(hostname, ".svc")

	parts := strings.Split(hostname, ".")
	if len(parts) < 1 {
		return nil, fmt.Errorf("unable to extract service from hostname %q", hostname)
	}

	service := parts[0]

	// Determine namespace
	namespace := ingressNS
	if len(parts) >= 2 {
		namespace = parts[1]
	}

	// Port
	var port int32
	if portStr != "" {
		var parsed int
		fmt.Sscanf(portStr, "%d", &parsed)
		port = int32(parsed)
	} else {
		switch u.Scheme {
		case "https":
			port = 443
		default:
			port = 80
		}
	}

	return &parsedAuthURL{
		service:   service,
		namespace: namespace,
		port:      port,
		path:      path,
		external:  false,
	}, nil
}
