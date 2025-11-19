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
	Buffer *resource.Quantity
	Cors   *CorsPolicy

	RuleBackendSources []PolicyIndex
}
