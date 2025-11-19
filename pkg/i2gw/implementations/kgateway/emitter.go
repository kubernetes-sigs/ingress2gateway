package kgateway

import (
	"time"

	kgwv1a1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	Name = "kgateway"
)

// KgatewayEmitter implements ImplementationEmitter and generates Kgateway
// resources from the merged IR, using provider output as the source.
type KgatewayEmitter struct{}

// Ensure KgatewayEmitter satisfies the ImplementationEmitter interface.
var _ i2gw.ImplementationEmitter = &KgatewayEmitter{}

func init() {
	// Register the kgateway emitter.
	i2gw.ImplementationEmitters[Name] = NewKgatewayEmitter()
}

// NewKgatewayEmitter returns a new instance of KgatewayEmitter.
func NewKgatewayEmitter() i2gw.ImplementationEmitter {
	return &KgatewayEmitter{}
}

// Name returns the name of the kgateway implementation.
func (e *KgatewayEmitter) Name() string {
	return Name
}

// Emit consumes the IR and returns Kgateway-specific resources as client.Objects.
// This implementation treats providers (e.g. ingress-nginx) as sources that populate
// provider-specific IR. It reads generic Policies from the provider IR and turns them
// into Kgateway resources and/or mutates the given IR. Whole-route policies are attached
// via targetRefs; partial policies are attached as ExtensionRef filters on backendRefs.
func (e *KgatewayEmitter) Emit(ir *intermediate.IR) ([]client.Object, error) {
	var out []client.Object
	var errs field.ErrorList

	for httpRouteKey, httpRouteContext := range ir.HTTPRoutes {
		ingx := httpRouteContext.ProviderSpecificIR.IngressNginx
		if ingx == nil {
			continue
		}

		// One TrafficPolicy per source Ingress name.
		tp := map[string]*kgwv1a1.TrafficPolicy{}

		for polSourceIngressName, pol := range ingx.Policies {
			// Apply feature-specific projections (buffer, CORS, etc.).
			touched := false

			if applyBufferPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, tp) {
				touched = true
			}
			if applyCorsPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, tp) {
				touched = true
			}
			if applyRateLimitPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, tp) {
				touched = true
			}

			if !touched {
				// No TrafficPolicy fields set for this policy; skip coverage wiring.
				continue
			}

			t := tp[polSourceIngressName]
			if t == nil {
				// Should not happen, but guard just in case.
				continue
			}

			// Coverage logic is shared across all features:
			// - If this policy covers all route backends, attach via targetRefs.
			// - Otherwise, attach via ExtensionRef filters on the covered backendRefs.
			if len(pol.RuleBackendSources) == numRules(httpRouteContext.HTTPRoute) {
				// Full coverage via targetRefs.
				t.Spec.TargetRefs = []kgwv1a1.LocalPolicyTargetReferenceWithSectionName{{
					LocalPolicyTargetReference: kgwv1a1.LocalPolicyTargetReference{
						Name: gwv1.ObjectName(httpRouteKey.Name),
					},
				}}
			} else {
				// Partial coverage via ExtensionRef filters on backendRefs.
				for _, idx := range pol.RuleBackendSources {
					httpRouteContext.Spec.Rules[idx.Rule].BackendRefs[idx.Backend].Filters =
						append(
							httpRouteContext.Spec.Rules[idx.Rule].BackendRefs[idx.Backend].Filters,
							gwv1.HTTPRouteFilter{
								Type: gwv1.HTTPRouteFilterExtensionRef,
								ExtensionRef: &gwv1.LocalObjectReference{
									Group: gwv1.Group(TrafficPolicyGVK.Group),
									Kind:  gwv1.Kind(TrafficPolicyGVK.Kind),
									Name:  gwv1.ObjectName(t.Name),
								},
							},
						)
				}
			}
		}

		// Write back the mutated HTTPRouteContext into the IR.
		ir.HTTPRoutes[httpRouteKey] = httpRouteContext

		// Collect TrafficPolicies for this HTTPRoute.
		for _, tp := range tp {
			out = append(out, tp)
		}
	}

	if len(errs) > 0 {
		return out, i2gw.AggregatedErrs(errs)
	}
	return out, nil
}

// applyBufferPolicy projects the Buffer policy IR into a Kgateway TrafficPolicy,
// returning true if it modified/created a TrafficPolicy for this ingress.
func applyBufferPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgwv1a1.TrafficPolicy,
) bool {
	if pol.Buffer == nil {
		return false
	}

	t := ensureTrafficPolicy(tp, ingressName, namespace)
	t.Spec.Buffer = &kgwv1a1.Buffer{
		MaxRequestSize: pol.Buffer,
	}
	return true
}

// applyCorsPolicy projects the CORS policy IR into a Kgateway TrafficPolicy,
// returning true if it modified/created a TrafficPolicy for the given ingress.
func applyCorsPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgwv1a1.TrafficPolicy,
) bool {
	if pol.Cors == nil || !pol.Cors.Enable || len(pol.Cors.AllowOrigin) == 0 {
		return false
	}

	// Dedupe origins while preserving order.
	seen := make(map[string]struct{}, len(pol.Cors.AllowOrigin))
	var origins []gwv1.CORSOrigin
	for _, o := range pol.Cors.AllowOrigin {
		if o == "" {
			continue
		}
		if _, ok := seen[o]; ok {
			continue
		}
		seen[o] = struct{}{}
		origins = append(origins, gwv1.CORSOrigin(o))
	}
	if len(origins) == 0 {
		return false
	}

	t := ensureTrafficPolicy(tp, ingressName, namespace)

	if t.Spec.Cors == nil {
		t.Spec.Cors = &kgwv1a1.CorsPolicy{}
	}
	if t.Spec.Cors.HTTPCORSFilter == nil {
		t.Spec.Cors.HTTPCORSFilter = &gwv1.HTTPCORSFilter{}
	}

	t.Spec.Cors.HTTPCORSFilter.AllowOrigins = origins
	return true
}

// ensureTrafficPolicy returns the TrafficPolicy for the given ingressName,
// creating and initializing it if needed.
func ensureTrafficPolicy(
	tp map[string]*kgwv1a1.TrafficPolicy,
	ingressName, namespace string,
) *kgwv1a1.TrafficPolicy {
	if existing, ok := tp[ingressName]; ok {
		return existing
	}

	newTP := &kgwv1a1.TrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
		Spec: kgwv1a1.TrafficPolicySpec{},
	}
	newTP.SetGroupVersionKind(TrafficPolicyGVK)

	tp[ingressName] = newTP
	return newTP
}

func numRules(hr gwv1.HTTPRoute) int {
	n := 0
	for _, r := range hr.Spec.Rules {
		n += len(r.BackendRefs)
	}
	return n
}

func applyRateLimitPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgwv1a1.TrafficPolicy,
) bool {
	if pol.RateLimit == nil {
		return false
	}

	rl := pol.RateLimit
	if rl.Limit <= 0 {
		return false
	}

	// Default burst multiplier to 1 if unset/zero.
	burstMult := rl.BurstMultiplier
	if burstMult <= 0 {
		burstMult = 1
	}

	var (
		maxTokens     int32
		tokensPerFill int32
		fillInterval  metav1.Duration
	)

	switch rl.Unit {
	case intermediate.RateLimitUnitRPS:
		// Requests per second.
		tokensPerFill = rl.Limit
		maxTokens = rl.Limit * burstMult
		fillInterval = metav1.Duration{Duration: time.Second}
	case intermediate.RateLimitUnitRPM:
		// Requests per minute.
		tokensPerFill = rl.Limit
		maxTokens = rl.Limit * burstMult
		fillInterval = metav1.Duration{Duration: time.Minute}
	default:
		// Unknown unit; ignore for now.
		return false
	}

	t := ensureTrafficPolicy(tp, ingressName, namespace)

	if t.Spec.RateLimit == nil {
		t.Spec.RateLimit = &kgwv1a1.RateLimit{}
	}
	if t.Spec.RateLimit.Local == nil {
		t.Spec.RateLimit.Local = &kgwv1a1.LocalRateLimitPolicy{}
	}

	// Helper to create *int32 without extra imports.
	int32Ptr := func(v int32) *int32 { return &v }

	t.Spec.RateLimit.Local.TokenBucket = &kgwv1a1.TokenBucket{
		MaxTokens:     maxTokens,
		TokensPerFill: int32Ptr(tokensPerFill),
		FillInterval:  fillInterval,
	}

	return true
}
