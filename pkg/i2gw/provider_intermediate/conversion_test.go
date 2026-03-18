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

package providerir

import (
	"testing"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestToEmitterIRConvertsIngressNginxPolicy(t *testing.T) {
	routeKey := types.NamespacedName{Namespace: "default", Name: "route"}
	backendKey := types.NamespacedName{Namespace: "default", Name: "backend-a"}
	useRegex := true
	backendProtocol := BackendProtocolGRPC

	sourceIR := ProviderIR{
		HTTPRoutes: map[types.NamespacedName]HTTPRouteContext{
			routeKey: {
				HTTPRoute: gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{Namespace: routeKey.Namespace, Name: routeKey.Name},
					Spec: gatewayv1.HTTPRouteSpec{
						Rules: []gatewayv1.HTTPRouteRule{{
							BackendRefs: []gatewayv1.HTTPBackendRef{{
								BackendRef: gatewayv1.BackendRef{
									BackendObjectReference: gatewayv1.BackendObjectReference{
										Name: "backend-a",
									},
								},
							}},
						}},
					},
				},
				ProviderSpecificIR: ProviderSpecificHTTPRouteIR{
					IngressNginx: &IngressNginxHTTPRouteIR{
						Policies: map[string]Policy{
							"ing-a": {
								Cors: &CorsPolicy{
									Enable:      true,
									AllowOrigin: []string{"https://example.com"},
								},
								RateLimit: &RateLimitPolicy{
									Limit:           10,
									Unit:            RateLimitUnitRPM,
									BurstMultiplier: 3,
								},
								UseRegexPaths: &useRegex,
								RuleBackendSources: []PolicyIndex{
									{Rule: 0, Backend: 0},
								},
								Backends: map[types.NamespacedName]Backend{
									backendKey: {
										Namespace: backendKey.Namespace,
										Name:      backendKey.Name,
										Port:      8080,
										Host:      "backend-a.default.svc.cluster.local",
										Protocol:  &backendProtocol,
									},
								},
							},
						},
						RegexLocationForHost:  ptr.To(true),
						RegexForcedByUseRegex: true,
					},
				},
				RuleBackendSources: [][]BackendSource{
					{{
						Ingress: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "ing-a"}},
					}},
				},
			},
		},
	}

	converted := ToEmitterIR(sourceIR)
	routeCtx, ok := converted.HTTPRoutes[routeKey]
	if !ok {
		t.Fatalf("expected converted HTTPRoute %v", routeKey)
	}
	if routeCtx.RegexLocationForHost == nil || !*routeCtx.RegexLocationForHost {
		t.Fatalf("expected RegexLocationForHost=true, got %#v", routeCtx.RegexLocationForHost)
	}

	pol, ok := routeCtx.PoliciesBySourceIngressName["ing-a"]
	if !ok {
		t.Fatalf("expected policy for ingress ing-a")
	}
	if pol.RateLimit == nil || pol.RateLimit.Unit != emitterir.RateLimitUnitRPM {
		t.Fatalf("expected rate limit unit %q, got %#v", emitterir.RateLimitUnitRPM, pol.RateLimit)
	}
	if pol.UseRegexPaths == nil || !*pol.UseRegexPaths {
		t.Fatalf("expected UseRegexPaths=true")
	}
	if len(pol.RuleBackendSources) != 1 || pol.RuleBackendSources[0].Rule != 0 || pol.RuleBackendSources[0].Backend != 0 {
		t.Fatalf("unexpected RuleBackendSources: %#v", pol.RuleBackendSources)
	}
	backend, ok := pol.Backends[backendKey]
	if !ok {
		t.Fatalf("expected backend %v", backendKey)
	}
	if backend.Protocol == nil || *backend.Protocol != emitterir.BackendProtocolGRPC {
		t.Fatalf("expected backend protocol %q, got %#v", emitterir.BackendProtocolGRPC, backend.Protocol)
	}

	// Ensure slice/map fields are copied, not shared.
	sourcePol := sourceIR.HTTPRoutes[routeKey].ProviderSpecificIR.IngressNginx.Policies["ing-a"]
	sourcePol.Cors.AllowOrigin[0] = "https://mutated.example.com"
	sourceIR.HTTPRoutes[routeKey].ProviderSpecificIR.IngressNginx.Policies["ing-a"] = sourcePol
	if got := pol.Cors.AllowOrigin[0]; got != "https://example.com" {
		t.Fatalf("expected converted policy to retain original allow origin, got %q", got)
	}

	// Ensure converted policies retain dedupe behavior for later emitter updates.
	updated := pol.AddRuleBackendSources([]emitterir.PolicyIndex{{Rule: 0, Backend: 0}, {Rule: 1, Backend: 0}})
	if len(updated.RuleBackendSources) != 2 {
		t.Fatalf("expected deduped rule/backend sources length 2, got %d", len(updated.RuleBackendSources))
	}
}
