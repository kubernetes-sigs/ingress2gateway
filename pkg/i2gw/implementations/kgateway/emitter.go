/*
Copyright 2023 The Kubernetes Authors.

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

package kgateway

import (
	"fmt"
	"strings"
	"time"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/notifications"
	kgwv1a1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// Name is the name of the kgateway implementation.
	Name = "kgateway"
)

// Emitter implements ImplementationEmitter and generates Kgateway
// resources from the merged IR, using provider output as the source.
type Emitter struct{}

// Ensure KgatewayEmitter satisfies the ImplementationEmitter interface.
var _ i2gw.ImplementationEmitter = &Emitter{}

func init() {
	// Register the kgateway emitter.
	i2gw.ImplementationEmitters[Name] = NewKgatewayEmitter()
}

// NewKgatewayEmitter returns a new instance of KgatewayEmitter.
func NewKgatewayEmitter() i2gw.ImplementationEmitter {
	return &Emitter{}
}

// Name returns the name of the kgateway implementation.
func (e *Emitter) Name() string {
	return Name
}

// Emit consumes the IR and returns Kgateway-specific resources as client.Objects.
// This implementation treats providers (e.g. ingress-nginx) as sources that populate
// provider-specific IR. It reads generic Policies from the provider IR and turns them
// into Kgateway resources and/or mutates the given IR. Whole-route policies are attached
// via targetRefs; partial policies are attached as ExtensionRef filters on backendRefs.
func (e *Emitter) Emit(ir *intermediate.IR) ([]client.Object, error) {
	var out []client.Object

	// One BackendConfigPolicy per Ingress name (per namespace), aggregating all
	// Services that Ingress routes to, when BackendConfigPolicy applicable is set.
	backendCfg := map[types.NamespacedName]*kgwv1a1.BackendConfigPolicy{}
	svcTimeouts := map[types.NamespacedName]map[string]*metav1.Duration{}

	for httpRouteKey, httpRouteContext := range ir.HTTPRoutes {
		ingx := httpRouteContext.ProviderSpecificIR.IngressNginx
		if ingx == nil {
			continue
		}

		// One TrafficPolicy per source Ingress name.
		tp := map[string]*kgwv1a1.TrafficPolicy{}

		for polSourceIngressName, pol := range ingx.Policies {
			// Normalize (rule, backend) coverage to unique pairs to avoid
			// generating duplicate filters on the same backendRef.
			coverage := uniquePolicyIndices(pol.RuleBackendSources)

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
			if applyTimeoutPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, tp) {
				touched = true
			}

			// Apply proxy-connect-timeout via BackendConfigPolicy.
			// Note: "touched" is not updated here, as this does not affect TrafficPolicy.
			applyProxyConnectTimeoutPolicy(
				pol,
				polSourceIngressName,
				httpRouteKey,
				httpRouteContext,
				backendCfg,
				svcTimeouts,
			)

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
			if len(coverage) == numRules(httpRouteContext.HTTPRoute) {
				// Full coverage via targetRefs.
				t.Spec.TargetRefs = []kgwv1a1.LocalPolicyTargetReferenceWithSectionName{{
					LocalPolicyTargetReference: kgwv1a1.LocalPolicyTargetReference{
						Name: gwv1.ObjectName(httpRouteKey.Name),
					},
				}}
			} else {
				// Partial coverage via ExtensionRef filters on backendRefs.
				for _, idx := range coverage {
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

	// Collect all BackendConfigPolicies computed across HTTPRoutes.
	for _, bcp := range backendCfg {
		out = append(out, bcp)
	}

	// Emit warnings for conflicting service timeouts
	for svc, ingressMap := range svcTimeouts {
		if len(ingressMap) <= 1 {
			continue
		}

		// Build message
		parts := []string{}
		for ing, d := range ingressMap {
			parts = append(parts, fmt.Sprintf("%s=%s", ing, d.Duration))
		}

		msg := fmt.Sprintf(
			"Multiple Ingresses set conflicting proxy-connect-timeout for Service %s/%s. Using lowest value. Values: %s",
			svc.Namespace,
			svc.Name,
			strings.Join(parts, ", "),
		)

		notifications.NotificationAggr.DispatchNotification(
			notifications.NewNotification(
				notifications.WarningNotification,
				msg,
			),
			"ingress-nginx",
		)
	}

	return out, nil
}

// uniquePolicyIndices returns a slice of PolicyIndex values with duplicates
// removed. Uniqueness is defined by the (Rule, Backend) pair.
//
// This is used to ensure we only attach a single TrafficPolicy ExtensionRef
// filter per backendRef for a given policy, even if the provider populated
// RuleBackendSources with duplicate entries for the same (rule, backend).
func uniquePolicyIndices(indices []intermediate.PolicyIndex) []intermediate.PolicyIndex {
	if len(indices) == 0 {
		return indices
	}

	seen := make(map[intermediate.PolicyIndex]struct{}, len(indices))
	out := make([]intermediate.PolicyIndex, 0, len(indices))

	for _, idx := range indices {
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, idx)
	}
	return out
}

// applyBufferPolicy projects the buffer-related policy IR into a Kgateway TrafficPolicy,
// returning true if it modified/created a TrafficPolicy for this ingress.
//
// Semantics are as follows:
//   - If the "nginx.ingress.kubernetes.io/proxy-body-size" annotation is present, that value
//     is used as the effective max request size.
//   - Otherwise, if the "nginx.ingress.kubernetes.io/client-body-buffer-size" annotation is present,
//     that value is used.
//   - If neither is set, no Kgateway Buffer policy is emitted.
//
// Note: Kgateway's Buffer.MaxRequestSize has "max body size" semantics (413 on exceed),
// which matches NGINX's proxy-body-size more directly. client-body-buffer-size is
// treated as a fallback when proxy-body-size is not configured.
func applyBufferPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgwv1a1.TrafficPolicy,
) bool {
	if pol.ClientBodyBufferSize == nil && pol.ProxyBodySize == nil {
		return false
	}

	// Prefer proxy-body-size if present; otherwise fall back to client-body-buffer-size.
	size := pol.ProxyBodySize
	if size == nil {
		size = pol.ClientBodyBufferSize
	}
	if size == nil {
		return false
	}

	t := ensureTrafficPolicy(tp, ingressName, namespace)
	t.Spec.Buffer = &kgwv1a1.Buffer{
		MaxRequestSize: size,
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

// applyTimeoutPolicy projects the timeout-related policy IR into a Kgateway TrafficPolicy,
// returning true if it modified/created a TrafficPolicy for this ingress.
//
// Semantics:
//   - If ProxySendTimeout is set, it is mapped to the Request timeout in Kgateway.
//   - If ProxyReadTimeout is set, it is mapped to the StreamIdle timeout in Kgateway.
func applyTimeoutPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgwv1a1.TrafficPolicy,
) bool {
	if pol.ProxySendTimeout == nil && pol.ProxyReadTimeout == nil {
		return false
	}

	t := ensureTrafficPolicy(tp, ingressName, namespace)

	if t.Spec.Timeouts == nil {
		t.Spec.Timeouts = &kgwv1a1.Timeouts{}
	}

	// Map proxy-send-timeout → Timeouts.Request
	if pol.ProxySendTimeout != nil {
		// "last writer wins" semantics: latest policy overwrites previous value.
		t.Spec.Timeouts.Request = pol.ProxySendTimeout
	}

	// Map proxy-read-timeout → Timeouts.StreamIdle
	if pol.ProxyReadTimeout != nil {
		t.Spec.Timeouts.StreamIdle = pol.ProxyReadTimeout
	}

	return true
}

// applyProxyConnectTimeoutPolicy projects the ProxyConnectTimeout IR policy into one or more
// Kgateway BackendConfigPolicies.
//
// Semantics:
//   - We create at most one BackendConfigPolicy per (namespace, ingressName).
//   - That policy's Spec.ConnectTimeout is taken from the Policy.ProxyConnectTimeout.
//   - TargetRefs are populated with all core Service backends that this Policy covers
//     (based on RuleBackendSources).
func applyProxyConnectTimeoutPolicy(
	pol intermediate.Policy,
	ingressName string,
	httpRouteKey types.NamespacedName,
	httpRouteCtx intermediate.HTTPRouteContext,
	backendCfg map[types.NamespacedName]*kgwv1a1.BackendConfigPolicy,
	svcTimeouts map[types.NamespacedName]map[string]*metav1.Duration,
) bool {
	if pol.ProxyConnectTimeout == nil {
		return false
	}

	for _, idx := range pol.RuleBackendSources {
		if idx.Rule >= len(httpRouteCtx.Spec.Rules) {
			continue
		}
		rule := httpRouteCtx.Spec.Rules[idx.Rule]
		if idx.Backend >= len(rule.BackendRefs) {
			continue
		}

		br := rule.BackendRefs[idx.Backend]

		if br.BackendRef.Group != nil && *br.BackendRef.Group != "" {
			continue
		}
		if br.BackendRef.Kind != nil && *br.BackendRef.Kind != "Service" {
			continue
		}

		svcName := string(br.BackendRef.Name)
		if svcName == "" {
			continue
		}

		svcKey := types.NamespacedName{
			Namespace: httpRouteKey.Namespace,
			Name:      svcName,
		}

		// Track per-Service timeout contributors
		if svcTimeouts[svcKey] == nil {
			svcTimeouts[svcKey] = map[string]*metav1.Duration{}
		}
		svcTimeouts[svcKey][ingressName] = pol.ProxyConnectTimeout

		// Create or reuse BackendConfigPolicy per Service
		bcp, exists := backendCfg[svcKey]
		if !exists {
			bcp = &kgwv1a1.BackendConfigPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      svcName + "-connect-timeout",
					Namespace: httpRouteKey.Namespace,
				},
				Spec: kgwv1a1.BackendConfigPolicySpec{
					TargetRefs: []kgwv1a1.LocalPolicyTargetReference{
						{
							Group: "",
							Kind:  "Service",
							Name:  gwv1.ObjectName(svcName),
						},
					},
					ConnectTimeout: pol.ProxyConnectTimeout,
				},
			}
			bcp.SetGroupVersionKind(BackendConfigPolicyGVK)
			backendCfg[svcKey] = bcp
		} else {
			// enforce "lowest timeout wins"
			cur := bcp.Spec.ConnectTimeout.Duration
			next := pol.ProxyConnectTimeout.Duration
			if next < cur {
				bcp.Spec.ConnectTimeout = pol.ProxyConnectTimeout
			}
		}
	}

	return true
}
