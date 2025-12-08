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
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/kgateway"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/shared"
	"k8s.io/utils/ptr"

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
	backendCfg := map[types.NamespacedName]*kgateway.BackendConfigPolicy{}
	svcTimeouts := map[types.NamespacedName]map[string]*metav1.Duration{}

	// Track HTTPListenerPolicies per Gateway (for access logging).
	httpListenerPolicies := map[types.NamespacedName]*kgateway.HTTPListenerPolicy{}

	// Track GatewayExtensions per auth URL (for external auth).
	gatewayExtensions := map[string]*kgateway.GatewayExtension{}

	// Track Backends per auth URL hostname (for static backends).
	backends := map[string]*kgateway.Backend{}

	for httpRouteKey, httpRouteContext := range ir.HTTPRoutes {
		ingx := httpRouteContext.ProviderSpecificIR.IngressNginx
		if ingx == nil {
			continue
		}

		// One TrafficPolicy per source Ingress name.
		tp := map[string]*kgateway.TrafficPolicy{}

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

			// Apply session affinity via BackendConfigPolicy.
			// Note: "touched" is not updated here, as this does not affect TrafficPolicy.
			applySessionAffinityPolicy(
				pol,
				httpRouteKey,
				httpRouteContext,
				backendCfg,
			)

			// Apply enable-access-log via HTTPListenerPolicy.
			applyAccessLogPolicy(
				pol,
				httpRouteKey,
				httpRouteContext,
				httpListenerPolicies,
			)

			// Apply auth-url via GatewayExtension and ExtAuthPolicy.
			if applyExtAuthPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, tp, gatewayExtensions, backends) {
				touched = true
			}

			// Apply basic auth via TrafficPolicy.
			if applyBasicAuthPolicy(pol, polSourceIngressName, httpRouteKey.Namespace, tp) {
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
			if len(coverage) == numRules(httpRouteContext.HTTPRoute) {
				// Full coverage via targetRefs.
				t.Spec.TargetRefs = []shared.LocalPolicyTargetReferenceWithSectionName{{
					LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
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

	// Collect all HTTPListenerPolicies computed across HTTPRoutes.
	for _, hlp := range httpListenerPolicies {
		out = append(out, hlp)
	}

	// Collect all GatewayExtensions computed across HTTPRoutes.
	for _, ge := range gatewayExtensions {
		out = append(out, ge)
	}

	// Collect all Backends computed across HTTPRoutes.
	for _, backend := range backends {
		out = append(out, backend)
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

	// Sort by Kind, then Namespace, then Name to make output deterministic for testing.
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := out[i], out[j]

		gvki := oi.GetObjectKind().GroupVersionKind()
		gvkj := oj.GetObjectKind().GroupVersionKind()

		ki, kj := gvki.Kind, gvkj.Kind
		if ki != kj {
			return ki < kj
		}

		nsi, nsj := oi.GetNamespace(), oj.GetNamespace()
		if nsi != nsj {
			return nsi < nsj
		}

		return oi.GetName() < oj.GetName()
	})

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
	tp map[string]*kgateway.TrafficPolicy,
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
	t.Spec.Buffer = &kgateway.Buffer{
		MaxRequestSize: size,
	}
	return true
}

// applyCorsPolicy projects the CORS policy IR into a Kgateway TrafficPolicy,
// returning true if it modified/created a TrafficPolicy for the given ingress.
func applyCorsPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgateway.TrafficPolicy,
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
		t.Spec.Cors = &kgateway.CorsPolicy{}
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
	tp map[string]*kgateway.TrafficPolicy,
	ingressName, namespace string,
) *kgateway.TrafficPolicy {
	if existing, ok := tp[ingressName]; ok {
		return existing
	}

	newTP := &kgateway.TrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
		Spec: kgateway.TrafficPolicySpec{},
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
	tp map[string]*kgateway.TrafficPolicy,
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
		t.Spec.RateLimit = &kgateway.RateLimit{}
	}
	if t.Spec.RateLimit.Local == nil {
		t.Spec.RateLimit.Local = &kgateway.LocalRateLimitPolicy{}
	}

	// Helper to create *int32 without extra imports.
	int32Ptr := func(v int32) *int32 { return &v }

	t.Spec.RateLimit.Local.TokenBucket = &kgateway.TokenBucket{
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
	tp map[string]*kgateway.TrafficPolicy,
) bool {
	if pol.ProxySendTimeout == nil && pol.ProxyReadTimeout == nil {
		return false
	}

	t := ensureTrafficPolicy(tp, ingressName, namespace)

	if t.Spec.Timeouts == nil {
		t.Spec.Timeouts = &shared.Timeouts{}
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
	backendCfg map[types.NamespacedName]*kgateway.BackendConfigPolicy,
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
			// Use a generic name that works for all backend config features
			policyName := svcName + "-backend-config"
			bcp = &kgateway.BackendConfigPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyName,
					Namespace: httpRouteKey.Namespace,
				},
				Spec: kgateway.BackendConfigPolicySpec{
					TargetRefs: []shared.LocalPolicyTargetReference{
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

// applySessionAffinityPolicy projects the SessionAffinity IR policy into one or more
// Kgateway BackendConfigPolicies.
//
// Semantics:
//   - We create at most one BackendConfigPolicy per Service.
//   - That policy's Spec.LoadBalancer.RingHash.HashPolicies is configured with cookie-based
//     session affinity settings from the Policy.SessionAffinity.
//   - TargetRefs are populated with all core Service backends that this Policy covers
//     (based on RuleBackendSources).
func applySessionAffinityPolicy(
	pol intermediate.Policy,
	httpRouteKey types.NamespacedName,
	httpRouteCtx intermediate.HTTPRouteContext,
	backendCfg map[types.NamespacedName]*kgateway.BackendConfigPolicy,
) bool {
	if pol.SessionAffinity == nil {
		return false
	}

	sessionAffinity := pol.SessionAffinity

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

		// Create or reuse BackendConfigPolicy per Service
		bcp, exists := backendCfg[svcKey]
		if !exists {
			// Determine policy name - use a generic name that works for both
			// session affinity and other backend config features
			policyName := svcName + "-backend-config"
			bcp = &kgateway.BackendConfigPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyName,
					Namespace: httpRouteKey.Namespace,
				},
				Spec: kgateway.BackendConfigPolicySpec{
					TargetRefs: []shared.LocalPolicyTargetReference{
						{
							Group: "",
							Kind:  "Service",
							Name:  gwv1.ObjectName(svcName),
						},
					},
				},
			}
			bcp.SetGroupVersionKind(BackendConfigPolicyGVK)
			backendCfg[svcKey] = bcp
		}

		// Build hash policy with cookie configuration
		cookieHashPolicy := &kgateway.Cookie{
			Name: sessionAffinity.CookieName,
		}

		if sessionAffinity.CookiePath != "" {
			cookieHashPolicy.Path = ptr.To(sessionAffinity.CookiePath)
		}

		if sessionAffinity.CookieExpires != nil {
			cookieHashPolicy.TTL = sessionAffinity.CookieExpires
		}

		if sessionAffinity.CookieSecure != nil {
			cookieHashPolicy.Secure = ptr.To(*sessionAffinity.CookieSecure)
		}

		if sessionAffinity.CookieSameSite != "" {
			cookieHashPolicy.SameSite = ptr.To(sessionAffinity.CookieSameSite)
		}

		// Set loadBalancer.ringHash.hashPolicies
		if bcp.Spec.LoadBalancer == nil {
			bcp.Spec.LoadBalancer = &kgateway.LoadBalancer{}
		}
		if bcp.Spec.LoadBalancer.RingHash == nil {
			bcp.Spec.LoadBalancer.RingHash = &kgateway.LoadBalancerRingHashConfig{}
		}
		// Replace existing hash policies with the new cookie-based one
		// (only one hash policy per service is typically needed)
		bcp.Spec.LoadBalancer.RingHash.HashPolicies = []kgateway.HashPolicy{
			{
				Cookie: cookieHashPolicy,
			},
		}
	}

	return true
}

// applyAccessLogPolicy projects the EnableAccessLog IR policy into one or more
// Kgateway HTTPListenerPolicies.
//
// Semantics:
//   - We create at most one HTTPListenerPolicy per Gateway (identified by ParentRefs).
//   - That policy's Spec.AccessLog is configured with FileSink when EnableAccessLog is true.
//   - TargetRefs are populated with the Gateway reference from HTTPRoute's ParentRefs.
func applyAccessLogPolicy(
	pol intermediate.Policy,
	httpRouteKey types.NamespacedName,
	httpRouteCtx intermediate.HTTPRouteContext,
	httpListenerPolicies map[types.NamespacedName]*kgateway.HTTPListenerPolicy,
) bool {
	if pol.EnableAccessLog == nil || !*pol.EnableAccessLog {
		return false
	}

	// Get Gateway references from HTTPRoute ParentRefs.
	if len(httpRouteCtx.Spec.ParentRefs) == 0 {
		return false
	}

	// Process each ParentRef (Gateway reference).
	for _, parentRef := range httpRouteCtx.Spec.ParentRefs {
		// Determine Gateway namespace (defaults to HTTPRoute namespace if not specified).
		gatewayNamespace := httpRouteKey.Namespace
		if parentRef.Namespace != nil {
			gatewayNamespace = string(*parentRef.Namespace)
		}

		gatewayName := string(parentRef.Name)
		if gatewayName == "" {
			continue
		}

		gatewayKey := types.NamespacedName{
			Namespace: gatewayNamespace,
			Name:      gatewayName,
		}

		// Create HTTPListenerPolicy per Gateway if it doesn't exist.
		if _, exists := httpListenerPolicies[gatewayKey]; !exists {
			hlp := &kgateway.HTTPListenerPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayName + "-access-log",
					Namespace: gatewayNamespace,
				},
				Spec: kgateway.HTTPListenerPolicySpec{
					TargetRefs: []shared.LocalPolicyTargetReference{
						{
							Group: "",
							Kind:  "Gateway",
							Name:  gwv1.ObjectName(gatewayName),
						},
					},
					AccessLog: []kgateway.AccessLog{
						{
							FileSink: &kgateway.FileSink{
								Path:         "/dev/stdout",
								StringFormat: ptr.To(`[%START_TIME%] "%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%" %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT% %DURATION% %RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)% "%REQ(X-FORWARDED-FOR)%" "%REQ(USER-AGENT)%" "%REQ(X-REQUEST-ID)%" "%REQ(:AUTHORITY)%" "%UPSTREAM_HOST%"%n`),
							},
						},
					},
				},
			}
			hlp.SetGroupVersionKind(HTTPListenerPolicyGVK)
			httpListenerPolicies[gatewayKey] = hlp
		}
		// If policy already exists, we don't need to modify it since access log is already enabled.
	}

	return true
}

// applyExtAuthPolicy projects the ExtAuth IR policy into a Kgateway Backend,
// GatewayExtension and ExtAuthPolicy in TrafficPolicy.
//
// Semantics:
//   - We create one Static Backend per unique hostname:port combination.
//   - We create one GatewayExtension per unique auth-url.
//   - That GatewayExtension's Spec.ExtAuth.HttpService references the Static Backend.
//   - An ExtAuthPolicy is added to TrafficPolicy that references the GatewayExtension.
func applyExtAuthPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgateway.TrafficPolicy,
	gatewayExtensions map[string]*kgateway.GatewayExtension,
	backends map[string]*kgateway.Backend,
) bool {
	if pol.ExtAuth == nil || pol.ExtAuth.AuthURL == "" {
		return false
	}

	authURL := pol.ExtAuth.AuthURL

	// Parse the URL to extract components.
	parsedURL, err := url.Parse(authURL)
	if err != nil {
		// Invalid URL, skip it.
		return false
	}

	// Extract hostname and port from the URL.
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return false
	}

	// Determine port (default to 80 for http, 443 for https).
	port := parsedURL.Port()
	portNum := int32(80) // default HTTP port
	if port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err == nil {
			portNum = int32(p)
		}
	} else if parsedURL.Scheme == "https" {
		portNum = 443
	}

	// Create a key for the backend based on hostname:port.
	backendKey := fmt.Sprintf("%s:%d", hostname, portNum)

	// Create Static Backend if it doesn't exist.
	if _, exists := backends[backendKey]; !exists {
		backend := &kgateway.Backend{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sanitizeNameForK8s(fmt.Sprintf("%s-backend", hostname)),
				Namespace: namespace,
			},
			Spec: kgateway.BackendSpec{
				Type: kgateway.BackendTypeStatic,
				Static: &kgateway.StaticBackend{
					Hosts: []kgateway.Host{
						{
							Host: hostname,
							Port: gwv1.PortNumber(portNum),
						},
					},
				},
			},
		}
		backend.SetGroupVersionKind(BackendGVK)
		backends[backendKey] = backend
	}

	// Use the URL as a key to deduplicate GatewayExtensions.
	if _, exists := gatewayExtensions[authURL]; !exists {
		// Extract path prefix from the URL.
		pathPrefix := parsedURL.Path
		if pathPrefix == "" {
			pathPrefix = "/"
		}

		backend := backends[backendKey]

		// Create GatewayExtension with ExtAuth using HttpService.
		extHttpService := &kgateway.ExtHttpService{
			BackendRef: gwv1.BackendRef{
				BackendObjectReference: gwv1.BackendObjectReference{
					Group: ptr.To(gwv1.Group("gateway.kgateway.dev")),
					Kind:  ptr.To(gwv1.Kind("Backend")),
					Name:  gwv1.ObjectName(backend.Name),
				},
			},
			PathPrefix: pathPrefix,
		}

		// Set AuthorizationResponse if response headers are specified.
		if len(pol.ExtAuth.ResponseHeaders) > 0 {
			extHttpService.AuthorizationResponse = &kgateway.AuthorizationResponse{
				HeadersToBackend: pol.ExtAuth.ResponseHeaders,
			}
		}

		ge := &kgateway.GatewayExtension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sanitizeNameForK8s(fmt.Sprintf("%s-extauth", hostname)),
				Namespace: namespace,
			},
			Spec: kgateway.GatewayExtensionSpec{
				ExtAuth: &kgateway.ExtAuthProvider{
					HttpService: extHttpService,
				},
			},
		}
		ge.SetGroupVersionKind(GatewayExtensionGVK)
		gatewayExtensions[authURL] = ge
	}

	// Add ExtAuthPolicy to TrafficPolicy.
	t := ensureTrafficPolicy(tp, ingressName, namespace)
	ge := gatewayExtensions[authURL]

	t.Spec.ExtAuth = &kgateway.ExtAuthPolicy{
		ExtensionRef: &shared.NamespacedObjectReference{
			Name:      gwv1.ObjectName(ge.Name),
			Namespace: ptr.To(gwv1.Namespace(ge.Namespace)),
		},
	}

	return true
}

// applyBasicAuthPolicy projects the BasicAuth IR policy into a Kgateway TrafficPolicy,
// returning true if it modified/created a TrafficPolicy for this ingress.
//
// Semantics:
//   - If BasicAuth is configured, set spec.basicAuth.secretRef.name in TrafficPolicy.
func applyBasicAuthPolicy(
	pol intermediate.Policy,
	ingressName, namespace string,
	tp map[string]*kgateway.TrafficPolicy,
) bool {
	if pol.BasicAuth == nil || pol.BasicAuth.SecretName == "" {
		return false
	}

	t := ensureTrafficPolicy(tp, ingressName, namespace)
	t.Spec.BasicAuth = &kgateway.BasicAuthPolicy{
		SecretRef: &kgateway.SecretReference{
			Name: gwv1.ObjectName(pol.BasicAuth.SecretName),
		},
	}
	return true
}

// sanitizeNameForK8s converts a string to a valid Kubernetes resource name.
// It replaces invalid characters with hyphens and ensures the name is valid.
func sanitizeNameForK8s(name string) string {
	// Replace invalid characters with hyphens.
	reg := strings.NewReplacer(
		".", "-",
		"_", "-",
		":", "-",
		"/", "-",
	)
	sanitized := reg.Replace(name)

	// Remove leading/trailing hyphens and ensure it starts with a letter or number.
	sanitized = strings.Trim(sanitized, "-")
	if len(sanitized) == 0 {
		sanitized = "extauth"
	}

	// Ensure it starts with a lowercase letter or number.
	if len(sanitized) > 0 && !((sanitized[0] >= 'a' && sanitized[0] <= 'z') || (sanitized[0] >= '0' && sanitized[0] <= '9')) {
		sanitized = "extauth-" + sanitized
	}

	// Limit length to 253 characters (Kubernetes limit).
	if len(sanitized) > 253 {
		sanitized = sanitized[:253]
	}

	return sanitized
}
