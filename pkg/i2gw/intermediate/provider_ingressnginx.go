package intermediate

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// ProxySendTimeout defines the timeout for transmitting a request to the proxied server.
	ProxySendTimeout *metav1.Duration

	// ProxyReadTimeout defines the timeout for reading a response from a proxied server.
	ProxyReadTimeout *metav1.Duration

	// ProxySendTimeout defines the timeout for establishing a connection to a proxied server.
	ProxyConnectTimeout *metav1.Duration

	RuleBackendSources []PolicyIndex

	// ruleBackendIndexSet is an internal helper used to deduplicate RuleBackendSources entries.
	ruleBackendIndexSet map[PolicyIndex]struct{}
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

// AddRuleBackendSources returns a copy of p with idxs added to
// RuleBackendSources, ensuring each (Rule, Backend) pair is unique.
func (p Policy) AddRuleBackendSources(idxs []PolicyIndex) Policy {
	pCopy := p

	// Initialize the internal set from any existing slice contents.
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
