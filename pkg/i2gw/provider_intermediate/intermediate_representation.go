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

package providerir

import (
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ProviderIR holds specifications of Gateway Objects for supporting Ingress extensions,
// annotations, and proprietary API features not supported as Gateway core
// features. An ProviderIR field can be mapped to core Gateway-API fields,
// or provider-specific Gateway extensions.
type ProviderIR struct {
	Gateways   map[types.NamespacedName]GatewayContext
	HTTPRoutes map[types.NamespacedName]HTTPRouteContext
	Services   map[types.NamespacedName]ProviderSpecificServiceIR

	GatewayClasses map[types.NamespacedName]gatewayv1.GatewayClass
	TLSRoutes      map[types.NamespacedName]gatewayv1alpha2.TLSRoute
	TCPRoutes      map[types.NamespacedName]gatewayv1alpha2.TCPRoute
	UDPRoutes      map[types.NamespacedName]gatewayv1alpha2.UDPRoute
	GRPCRoutes     map[types.NamespacedName]gatewayv1.GRPCRoute

	BackendTLSPolicies map[types.NamespacedName]gatewayv1.BackendTLSPolicy
	ReferenceGrants    map[types.NamespacedName]gatewayv1beta1.ReferenceGrant
}

// GatewayContext contains the Gateway-API Gateway object and GatewayIR, which
// has a dedicated field for each provider to specify their extension features
// on Gateways.
// The IR will contain necessary information to construct the Gateway
// extensions, but not the extensions themselves.
type GatewayContext struct {
	gatewayv1.Gateway
	ProviderSpecificIR ProviderSpecificGatewayIR
}

type ProviderSpecificGatewayIR struct {
	Gce          *gce.GatewayIR
	IngressNginx *IngressNginxGatewayIR
}

// HTTPRouteContext contains the Gateway-API HTTPRoute object and HTTPRouteIR,
// which has a dedicated field for each provider to specify their extension
// features on HTTPRoutes.
// The IR will contain necessary information to construct the HTTPRoute
// extensions, but not the extensions themselves.
type HTTPRouteContext struct {
	gatewayv1.HTTPRoute
	ProviderSpecificIR ProviderSpecificHTTPRouteIR

	// RuleBackendSources[i][j] is the source of the jth backend in the ith element of HTTPRoute.Spec.Rules.
	RuleBackendSources [][]BackendSource
}

type ProviderSpecificHTTPRouteIR struct {
	Gce          *gce.HTTPRouteIR
	IngressNginx *IngressNginxHTTPRouteIR
}

// ProviderSpecificServiceIR contains a dedicated field for each provider to specify their
// extension features on Service.
type ProviderSpecificServiceIR struct {
	Gce          *gce.ServiceIR
	IngressNginx *IngressNginxServiceIR
}

// IngressNginxGatewayIR is the provider-specific IR for ingress-nginx.
type IngressNginxGatewayIR struct{}

// IngressNginxHTTPRouteIR contains ingress-nginx-specific fields for HTTPRoute.
type IngressNginxHTTPRouteIR struct {
	// Policies keyed by source Ingress name.
	Policies map[string]IngressNginxPolicy

	// RegexLocationForHost is true when ingress-nginx would enforce the "~*" (case-insensitive)
	// regex location modifier for ALL paths under a host.
	//
	// Per nginx semantics, this becomes true if ANY ingress for the host has either of the
	// following: annotations:
	//
	//   - nginx.ingress.kubernetes.io/use-regex: "true"
	//   - nginx.ingress.kubernetes.io/rewrite-target set to any value
	RegexLocationForHost *bool

	// RegexForcedByUseRegex is true when RegexLocationForHost is true specifically
	// because of the nginx.ingress.kubernetes.io/use-regex annotation.
	RegexForcedByUseRegex bool

	// RegexForcedByRewrite is true when RegexLocationForHost is true specifically
	// because of the nginx.ingress.kubernetes.io/rewrite-target annotation.
	RegexForcedByRewrite bool
}

// IngressNginxServiceIR contains ingress-nginx-specific fields for Service.
type IngressNginxServiceIR struct{}

// IngressNginxPolicyIndex identifies a (rule, backend) pair within a merged HTTPRoute.
type IngressNginxPolicyIndex struct {
	Rule    int
	Backend int
}

// IngressNginxCorsPolicy defines a CORS policy that has been extracted from ingress-nginx annotations.
type IngressNginxCorsPolicy struct {
	// Enable corresponds to nginx.ingress.kubernetes.io/enable-cors and indicates whether CORS
	// is enabled.
	Enable bool

	// AllowOrigin corresponds to nginx.ingress.kubernetes.io/cors-allow-origin and controls what
	// is the accepted Origin for CORS.
	AllowOrigin []string

	// AllowCredentials corresponds to nginx.ingress.kubernetes.io/cors-allow-credentials and controls
	// if credentials can be passed during CORS operations. When nil, the provider has not specified a value.
	AllowCredentials *bool

	// AllowHeaders corresponds to nginx.ingress.kubernetes.io/cors-allow-headers and controls which
	// headers are accepted. Values are stored as raw header names; case-insensitivity is handled by consumers.
	AllowHeaders []string

	// ExposeHeaders corresponds to nginx.ingress.kubernetes.io/cors-expose-headers.
	// Values are header names as they appeared in the annotation, trimmed of
	// surrounding whitespace but otherwise case-preserving.
	ExposeHeaders []string

	// AllowMethods corresponds to nginx.ingress.kubernetes.io/cors-allow-methods and controls which methods
	// are accepted. Values are stored as raw method names; consumers can normalize/validate.
	AllowMethods []string

	// MaxAge corresponds to nginx.ingress.kubernetes.io/cors-max-age, in seconds and controls how long preflight
	// requests can be cached. When nil, the provider has not specified a value.
	MaxAge *int32
}

// IngressNginxExtAuthPolicy defines an external authentication policy that has been extracted from ingress-nginx annotations.
type IngressNginxExtAuthPolicy struct {
	// AuthURL defines the URL of an external authentication service.
	AuthURL string
	// ResponseHeaders defines the headers to pass to backend once authentication request completes.
	ResponseHeaders []string
}

// IngressNginxBasicAuthPolicy defines a basic authentication policy that has been extracted from ingress-nginx annotations.
type IngressNginxBasicAuthPolicy struct {
	// SecretName defines the name of the secret containing basic auth credentials.
	SecretName string
	// AuthType defines the format of the secret: "auth-file" (default) or "auth-map".
	// For "auth-file", the secret contains an htpasswd file in a specific key.
	// For "auth-map", the keys of the secret are usernames and values are hashed passwords.
	AuthType string
}

// IngressNginxSessionAffinityPolicy defines a session affinity policy that has been extracted from ingress-nginx annotations.
type IngressNginxSessionAffinityPolicy struct {
	// CookieName defines the name of the cookie used for session affinity.
	CookieName string
	// CookiePath defines the path that will be set on the cookie.
	CookiePath string
	// CookieDomain defines the Domain attribute of the sticky cookie.
	CookieDomain string
	// CookieSameSite defines the SameSite attribute of the sticky cookie (None, Lax, Strict).
	CookieSameSite string
	// CookieExpires defines the TTL/expiration time for the cookie.
	CookieExpires *metav1.Duration
	// CookieSecure defines whether the Secure flag is set on the cookie.
	CookieSecure *bool
}

// IngressNginxBackendTLSPolicy defines a backend TLS policy that has been extracted from ingress-nginx annotations.
type IngressNginxBackendTLSPolicy struct {
	// SecretName defines the name of the secret containing client certificate (tls.crt),
	// client key (tls.key), and CA certificate (ca.crt) in PEM format.
	// Format: "namespace/secretName"
	SecretName string
	// Verify enables or disables verification of the proxied HTTPS server certificate.
	// Default: false (off)
	Verify bool
	// Hostname allows overriding the server name used to verify the certificate of the proxied HTTPS server.
	// This value is also used for SNI when a connection is established.
	// In Gateway API, setting Hostname enables SNI automatically.
	Hostname string
}

// IngressNginxPolicy describes all per-Ingress policy knobs that ingress-nginx projects into the
// IR (buffer, CORS, etc.).
type IngressNginxPolicy struct {
	// ClientBodyBufferSize defines the size of the buffer used for client request bodies.
	ClientBodyBufferSize *resource.Quantity

	// ProxyBodySize defines the maximum allowed size of the client request body.
	ProxyBodySize *resource.Quantity

	// Cors defines the CORS policy derived from ingress-nginx annotations.
	Cors *IngressNginxCorsPolicy

	// RateLimit is a generic rate limit policy derived from ingress-nginx annotations.
	RateLimit *IngressNginxRateLimitPolicy

	// ProxySendTimeout defines the timeout for transmitting a request to the proxied server.
	ProxySendTimeout *metav1.Duration

	// ProxyReadTimeout defines the timeout for reading a response from a proxied server.
	ProxyReadTimeout *metav1.Duration

	// ProxyConnectTimeout defines the timeout for establishing a connection to a proxied server.
	ProxyConnectTimeout *metav1.Duration

	// EnableAccessLog defines whether access logging is enabled for the ingress.
	EnableAccessLog *bool

	// ExtAuth defines the external authentication policy.
	ExtAuth *IngressNginxExtAuthPolicy

	// BasicAuth defines the basic authentication policy.
	BasicAuth *IngressNginxBasicAuthPolicy

	// SessionAffinity defines the session affinity policy.
	SessionAffinity *IngressNginxSessionAffinityPolicy

	// LoadBalancing controls the upstream load-balancing algorithm. Only round_robin is supported;
	// other values are ignored.
	LoadBalancing *IngressNginxBackendLoadBalancingPolicy

	// BackendTLS defines the backend TLS policy.
	BackendTLS *IngressNginxBackendTLSPolicy

	// BackendProtocol defines the upstream application protocol to use when communicating with
	// backend Services covered by this policy.
	BackendProtocol *IngressNginxBackendProtocol

	// SSLRedirect indicates whether SSL redirect is enabled, corresponding to
	// nginx.ingress.kubernetes.io/ssl-redirect. When true, requests should be
	// redirected to HTTPS.
	SSLRedirect *bool

	// RewriteTarget corresponds to nginx.ingress.kubernetes.io/rewrite-target annotation and rewrites the
	// path in the request to the path expected by the service.
	RewriteTarget *string

	// UseRegexPaths corresponds to nginx.ingress.kubernetes.io/use-regex.
	// When true (and host-wide regex mode is enabled), paths contributed by this ingress
	// must be treated as regex patterns (i.e. NOT escaped as literals).
	UseRegexPaths *bool

	// RuleBackendSources lists the (rule, backend) pairs within a merged HTTPRoute
	// that this policy applies to.
	//
	// Each entry is a IngressNginxPolicyIndex struct identifying a (rule, backend) pair.
	//
	// This slice may contain duplicates; use AddRuleBackendSources to add entries
	// while ensuring uniqueness.
	RuleBackendSources []IngressNginxPolicyIndex

	// Backends holds all proxied backends that cannot be rendered as a standard k8s service, i.e. kgateway Backend.
	Backends map[types.NamespacedName]IngressNginxBackend

	// ruleBackendIndexSet is an internal helper used to deduplicate RuleBackendSources entries.
	ruleBackendIndexSet map[IngressNginxPolicyIndex]struct{}
}

// IngressNginxBackendProtocol defines the L7 protocol used to talk to a Backend.
type IngressNginxBackendProtocol string

// IngressNginxBackendProtocolGRPC is the gRPC protocol.
const IngressNginxBackendProtocolGRPC IngressNginxBackendProtocol = "grpc"

// IngressNginxBackend defines a proxied backend that cannot be rendered as a standard k8s Service.
type IngressNginxBackend struct {
	// Namespace defines the namespace of the backend.
	Namespace string

	// Name defines the name of the backend.
	Name string

	// Port defines the port of the backend.
	Port int32

	// Host defines the host (IP or DNS name) of the backend.
	Host string

	// Protocol defines the application protocol used to communicate with the backend.
	// When nil, the default HTTP/1.x semantics should be assumed by consumers.
	Protocol *IngressNginxBackendProtocol
}

// IngressNginxRateLimitUnit defines the unit of rate limiting.
type IngressNginxRateLimitUnit string

const (
	// IngressNginxRateLimitUnitRPS defines rate limit in requests per second.
	IngressNginxRateLimitUnitRPS IngressNginxRateLimitUnit = "rps"
	// IngressNginxRateLimitUnitRPM defines rate limit in requests per minute.
	IngressNginxRateLimitUnitRPM IngressNginxRateLimitUnit = "rpm"
)

// IngressNginxRateLimitPolicy defines a rate limiting policy derived from ingress-nginx annotations.
type IngressNginxRateLimitPolicy struct {
	// Exactly one of RPS/RPM should be set by the provider.
	Limit int32                     // normalized numeric limit
	Unit  IngressNginxRateLimitUnit // "rps" or "rpm"

	// BurstMultiplier is applied on top of the base limit to compute the bucket size.
	// If zero, treat as 1.
	BurstMultiplier int32
}

// IngressNginxLoadBalancingStrategy represents the upstream load-balancing mode requested by the Ingress NGINX annotations.
// Currently only round_robin is supported; other values are ignored.
type IngressNginxLoadBalancingStrategy string

const IngressNginxLoadBalancingStrategyRoundRobin IngressNginxLoadBalancingStrategy = "round_robin"

type IngressNginxBackendLoadBalancingPolicy struct {
	Strategy IngressNginxLoadBalancingStrategy
}

// AddRuleBackendSources returns a copy of p with idxs added to
// RuleBackendSources, ensuring each (Rule, Backend) pair is unique.
func (p IngressNginxPolicy) AddRuleBackendSources(idxs []IngressNginxPolicyIndex) IngressNginxPolicy {
	pCopy := p

	// Initialize the internal set from any existing slice contents.
	if len(pCopy.RuleBackendSources) > 0 && pCopy.ruleBackendIndexSet == nil {
		pCopy.ruleBackendIndexSet = make(map[IngressNginxPolicyIndex]struct{}, len(pCopy.RuleBackendSources))
		for _, existing := range pCopy.RuleBackendSources {
			pCopy.ruleBackendIndexSet[existing] = struct{}{}
		}
	}
	if pCopy.ruleBackendIndexSet == nil {
		pCopy.ruleBackendIndexSet = make(map[IngressNginxPolicyIndex]struct{})
	}

	for _, idx := range idxs {
		if _, exists := pCopy.ruleBackendIndexSet[idx]; exists {
			continue
		}
		pCopy.RuleBackendSources = append(pCopy.RuleBackendSources, idx)
		pCopy.ruleBackendIndexSet[idx] = struct{}{}
	}

	return pCopy
}

// Type aliases for backward compatibility with existing provider code.
// These allow providers to use shorter names while maintaining the prefixed names
// for clarity in the IR structure.

// Policy is an alias for IngressNginxPolicy
type Policy = IngressNginxPolicy

// PolicyIndex is an alias for IngressNginxPolicyIndex
type PolicyIndex = IngressNginxPolicyIndex

// CorsPolicy is an alias for IngressNginxCorsPolicy
type CorsPolicy = IngressNginxCorsPolicy

// ExtAuthPolicy is an alias for IngressNginxExtAuthPolicy
type ExtAuthPolicy = IngressNginxExtAuthPolicy

// BasicAuthPolicy is an alias for IngressNginxBasicAuthPolicy
type BasicAuthPolicy = IngressNginxBasicAuthPolicy

// SessionAffinityPolicy is an alias for IngressNginxSessionAffinityPolicy
type SessionAffinityPolicy = IngressNginxSessionAffinityPolicy

// BackendTLSPolicy is an alias for IngressNginxBackendTLSPolicy
type BackendTLSPolicy = IngressNginxBackendTLSPolicy

// BackendProtocol is an alias for IngressNginxBackendProtocol
type BackendProtocol = IngressNginxBackendProtocol

// BackendProtocolGRPC is an alias for IngressNginxBackendProtocolGRPC
const BackendProtocolGRPC = IngressNginxBackendProtocolGRPC

// Backend is an alias for IngressNginxBackend
type Backend = IngressNginxBackend

// RateLimitUnit is an alias for IngressNginxRateLimitUnit
type RateLimitUnit = IngressNginxRateLimitUnit

// RateLimitUnitRPS is an alias for IngressNginxRateLimitUnitRPS
const RateLimitUnitRPS = IngressNginxRateLimitUnitRPS

// RateLimitUnitRPM is an alias for IngressNginxRateLimitUnitRPM
const RateLimitUnitRPM = IngressNginxRateLimitUnitRPM

// RateLimitPolicy is an alias for IngressNginxRateLimitPolicy
type RateLimitPolicy = IngressNginxRateLimitPolicy

// LoadBalancingStrategy is an alias for IngressNginxLoadBalancingStrategy
type LoadBalancingStrategy = IngressNginxLoadBalancingStrategy

// LoadBalancingStrategyRoundRobin is an alias for IngressNginxLoadBalancingStrategyRoundRobin
const LoadBalancingStrategyRoundRobin = IngressNginxLoadBalancingStrategyRoundRobin

// BackendLoadBalancingPolicy is an alias for IngressNginxBackendLoadBalancingPolicy
type BackendLoadBalancingPolicy = IngressNginxBackendLoadBalancingPolicy

// BackendSource tracks the source Ingress resource that contributed
// a specific BackendRef to an HTTPRoute rule.
type BackendSource struct {
	// Source Ingress that contributed this backend
	Ingress *networkingv1.Ingress

	// Exactly one of Path or DefaultBackend must be non-nil.
	// Path points to the specific HTTPIngressPath that contributed this backend.
	Path *networkingv1.HTTPIngressPath

	// DefaultBackend points to the Ingress's spec.defaultBackend that contributed this backend.
	DefaultBackend *networkingv1.IngressBackend
}
