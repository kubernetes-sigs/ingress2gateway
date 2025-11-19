package intermediate

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// Provider-specific IR for ingress-nginx.
type IngressNginxGatewayIR struct{}

type IngressNginxHTTPRouteIR struct {
	// Policies keyed by source Ingress name.
	Policies map[string]Policy
}

type IngressNginxServiceIR struct{}

// PolicyIndex identifies a (rule, backend) pair within a merged HTTPRoute.
type PolicyIndex struct {
	Rule    int
	Backend int
}

// CorsPolicy defines a CORS policy that has been extracted from ingress-nginx annotations.
type CorsPolicy struct {
	Enable      bool
	AllowOrigin []string
}

// Policy describes all per-Ingress policy knobs that ingress-nginx projects into the
// IR (buffer, CORS, etc.).
type Policy struct {
	// ClientBodyBufferSize defines the size of the buffer used for client request bodies.
	ClientBodyBufferSize *resource.Quantity
	// ProxyBodySize defines the maximum allowed size of the client request body.
	ProxyBodySize *resource.Quantity
	Cors          *CorsPolicy

	// RateLimit is a generic rate limit policy derived from ingress-nginx annotations.
	RateLimit *RateLimitPolicy

	RuleBackendSources []PolicyIndex
}

type RateLimitUnit string

const (
	RateLimitUnitRPS RateLimitUnit = "rps"
	RateLimitUnitRPM RateLimitUnit = "rpm"
)

type RateLimitPolicy struct {
	// Exactly one of RPS/RPM should be set by the provider.
	Limit int32         // normalized numeric limit
	Unit  RateLimitUnit // "rps" or "rpm"

	// BurstMultiplier is applied on top of the base limit to compute the bucket size.
	// If zero, treat as 1.
	BurstMultiplier int32
}
