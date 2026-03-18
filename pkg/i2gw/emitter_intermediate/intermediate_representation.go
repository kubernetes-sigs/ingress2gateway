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

package emitterir

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

// EmitterIR holds specifications of Gateway Objects for supporting Ingress extensions,
// annotations, and proprietary API features not supported as Gateway core
// features. An EmitterIR field can be mapped to core Gateway-API fields,
// or provider-specific Gateway extensions.
type EmitterIR struct {
	Gateways   map[types.NamespacedName]GatewayContext
	HTTPRoutes map[types.NamespacedName]HTTPRouteContext

	GatewayClasses map[types.NamespacedName]GatewayClassContext
	TLSRoutes      map[types.NamespacedName]TLSRouteContext
	TCPRoutes      map[types.NamespacedName]TCPRouteContext
	UDPRoutes      map[types.NamespacedName]UDPRouteContext
	GRPCRoutes     map[types.NamespacedName]GRPCRouteContext

	BackendTLSPolicies map[types.NamespacedName]BackendTLSPolicyContext
	ReferenceGrants    map[types.NamespacedName]ReferenceGrantContext

	GceServices map[types.NamespacedName]gce.ServiceIR
}

type GatewayContext struct {
	gatewayv1.Gateway
	// Emitter IR should be provider/emitter neutral,
	// But we have GCE for backcompatibility.
	Gce *gce.GatewayIR
}

type HTTPRouteContext struct {
	gatewayv1.HTTPRoute

	// PoliciesBySourceIngressName stores feature policy data keyed by source Ingress name.
	PoliciesBySourceIngressName map[string]Policy

	// RegexLocationForHost is true when regex location matching should be used for the route host.
	RegexLocationForHost *bool

	// RegexForcedByUseRegex is true when RegexLocationForHost is driven by use-regex annotation.
	RegexForcedByUseRegex bool

	// RegexForcedByRewrite is true when RegexLocationForHost is driven by rewrite-target annotation.
	RegexForcedByRewrite bool

	// RuleBackendSources tracks the source Ingress resources for each backend.
	RuleBackendSources [][]BackendSource
}

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

type GatewayClassContext struct {
	gatewayv1.GatewayClass
}

type TLSRouteContext struct {
	gatewayv1alpha2.TLSRoute
}

type TCPRouteContext struct {
	gatewayv1alpha2.TCPRoute
}

type UDPRouteContext struct {
	gatewayv1alpha2.UDPRoute
}

type GRPCRouteContext struct {
	gatewayv1.GRPCRoute
}

type BackendTLSPolicyContext struct {
	gatewayv1.BackendTLSPolicy
}

type ReferenceGrantContext struct {
	gatewayv1beta1.ReferenceGrant
}

// PolicyIndex identifies a (rule, backend) pair within a merged HTTPRoute.
type PolicyIndex struct {
	Rule    int
	Backend int
}

// CorsPolicy defines a CORS policy extracted from annotations.
type CorsPolicy struct {
	Enable           bool
	AllowOrigin      []string
	AllowCredentials *bool
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowMethods     []string
	MaxAge           *int32
}

// ExtAuthPolicy defines an external auth policy extracted from annotations.
type ExtAuthPolicy struct {
	AuthURL         string
	ResponseHeaders []string
}

// BasicAuthPolicy defines a basic auth policy extracted from annotations.
type BasicAuthPolicy struct {
	SecretName string
	AuthType   string
}

// SessionAffinityPolicy defines a session affinity policy extracted from annotations.
type SessionAffinityPolicy struct {
	CookieName     string
	CookiePath     string
	CookieDomain   string
	CookieSameSite string
	CookieExpires  *metav1.Duration
	CookieSecure   *bool
}

// BackendTLSPolicy defines a backend TLS policy extracted from annotations.
type BackendTLSPolicy struct {
	SecretName string
	Verify     bool
	Hostname   string
}

// Policy describes per-Ingress policy knobs projected by providers.
type Policy struct {
	ClientBodyBufferSize *resource.Quantity
	ProxyBodySize        *resource.Quantity
	Cors                 *CorsPolicy
	RateLimit            *RateLimitPolicy
	ProxySendTimeout     *metav1.Duration
	ProxyReadTimeout     *metav1.Duration
	ProxyConnectTimeout  *metav1.Duration
	EnableAccessLog      *bool
	ExtAuth              *ExtAuthPolicy
	BasicAuth            *BasicAuthPolicy
	SessionAffinity      *SessionAffinityPolicy
	LoadBalancing        *BackendLoadBalancingPolicy
	BackendTLS           *BackendTLSPolicy
	BackendProtocol      *BackendProtocol
	SSLRedirect          *bool
	RewriteTarget        *string
	UseRegexPaths        *bool

	// RuleBackendSources lists covered (rule, backend) pairs in the merged HTTPRoute.
	RuleBackendSources []PolicyIndex

	// Backends holds all proxied backends that cannot be rendered as a standard k8s service.
	Backends map[types.NamespacedName]Backend

	// ruleBackendIndexSet is an internal helper used to deduplicate RuleBackendSources entries.
	ruleBackendIndexSet map[PolicyIndex]struct{}
}

// BackendProtocol defines the L7 protocol used to talk to a Backend.
type BackendProtocol string

// BackendProtocolGRPC is the gRPC protocol.
const BackendProtocolGRPC BackendProtocol = "grpc"

// Backend defines a proxied backend that cannot be rendered as a standard k8s Service.
type Backend struct {
	Namespace string
	Name      string
	Port      int32
	Host      string
	Protocol  *BackendProtocol
}

// RateLimitUnit defines the unit of rate limiting.
type RateLimitUnit string

const (
	// RateLimitUnitRPS defines rate limit in requests per second.
	RateLimitUnitRPS RateLimitUnit = "rps"
	// RateLimitUnitRPM defines rate limit in requests per minute.
	RateLimitUnitRPM RateLimitUnit = "rpm"
)

// RateLimitPolicy defines a rate limiting policy derived from annotations.
type RateLimitPolicy struct {
	Limit           int32
	Unit            RateLimitUnit
	BurstMultiplier int32
}

// LoadBalancingStrategy represents upstream load-balancing mode.
type LoadBalancingStrategy string

// LoadBalancingStrategyRoundRobin is the supported round_robin strategy.
const LoadBalancingStrategyRoundRobin LoadBalancingStrategy = "round_robin"

// BackendLoadBalancingPolicy defines backend load-balancing policy.
type BackendLoadBalancingPolicy struct {
	Strategy LoadBalancingStrategy
}

// AddRuleBackendSources returns a copy of p with idxs added to RuleBackendSources,
// ensuring each (rule, backend) pair is unique.
func (p Policy) AddRuleBackendSources(idxs []PolicyIndex) Policy {
	pCopy := p

	if len(pCopy.RuleBackendSources) > 0 && pCopy.ruleBackendIndexSet == nil {
		pCopy.ruleBackendIndexSet = make(map[PolicyIndex]struct{}, len(pCopy.RuleBackendSources))
		for _, existing := range pCopy.RuleBackendSources {
			pCopy.ruleBackendIndexSet[existing] = struct{}{}
		}
	}
	if pCopy.ruleBackendIndexSet == nil {
		pCopy.ruleBackendIndexSet = make(map[PolicyIndex]struct{})
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

// Backward-compatibility aliases for older ingress-nginx-prefixed names.

type IngressNginxPolicy = Policy

type IngressNginxPolicyIndex = PolicyIndex

type IngressNginxCorsPolicy = CorsPolicy

type IngressNginxExtAuthPolicy = ExtAuthPolicy

type IngressNginxBasicAuthPolicy = BasicAuthPolicy

type IngressNginxSessionAffinityPolicy = SessionAffinityPolicy

type IngressNginxBackendTLSPolicy = BackendTLSPolicy

type IngressNginxBackendProtocol = BackendProtocol

const IngressNginxBackendProtocolGRPC = BackendProtocolGRPC

type IngressNginxBackend = Backend

type IngressNginxRateLimitUnit = RateLimitUnit

const IngressNginxRateLimitUnitRPS = RateLimitUnitRPS

const IngressNginxRateLimitUnitRPM = RateLimitUnitRPM

type IngressNginxRateLimitPolicy = RateLimitPolicy

type IngressNginxLoadBalancingStrategy = LoadBalancingStrategy

const IngressNginxLoadBalancingStrategyRoundRobin = LoadBalancingStrategyRoundRobin

type IngressNginxBackendLoadBalancingPolicy = BackendLoadBalancingPolicy
