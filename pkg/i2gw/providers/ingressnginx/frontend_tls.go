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

package ingressnginx

import (
	"strings"
	"time"

	providerir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	frontendTLSHandshakeTimeoutAnnotation = "nginx.ingress.kubernetes.io/ssl-handshake-timeout"
	frontendTLSALPNProtocolsAnnotation    = "nginx.ingress.kubernetes.io/ssl-alpn"

	maxFrontendTLSALPNProtocols = 16
	defaultTLSHandshakeTimeout  = 15 * time.Second
)

// frontendTLSFeature parses frontend TLS listener settings from ingress-nginx annotations and
// records them into ingress-nginx policy IR.
func frontendTLSFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *providerir.ProviderIR,
) field.ErrorList {
	var errs field.ErrorList
	perIngress := map[types.NamespacedName]*providerir.IngressNginxFrontendTLSPolicy{}

	for i := range ingresses {
		ing := &ingresses[i]

		rawHandshake := strings.TrimSpace(ing.Annotations[frontendTLSHandshakeTimeoutAnnotation])
		rawALPN := strings.TrimSpace(ing.Annotations[frontendTLSALPNProtocolsAnnotation])
		if rawHandshake == "" && rawALPN == "" {
			continue
		}

		policy := &providerir.IngressNginxFrontendTLSPolicy{}

		if rawHandshake != "" {
			d, err := time.ParseDuration(rawHandshake)
			if err != nil {
				d, err = time.ParseDuration(rawHandshake + "s")
			}
			if err != nil {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(frontendTLSHandshakeTimeoutAnnotation),
					rawHandshake,
					"failed to parse ssl-handshake-timeout",
				))
				continue
			}
			if d < 100*time.Millisecond {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(frontendTLSHandshakeTimeoutAnnotation),
					rawHandshake,
					"ssl-handshake-timeout must be at least 100ms",
				))
				continue
			}
			policy.HandshakeTimeout = &metav1.Duration{Duration: d}
		}

		if rawALPN != "" {
			parts := strings.Split(rawALPN, ",")
			seen := map[string]struct{}{}
			for _, part := range parts {
				value := strings.TrimSpace(part)
				if value == "" {
					continue
				}
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				policy.ALPNProtocols = append(policy.ALPNProtocols, value)
			}
			if len(policy.ALPNProtocols) == 0 {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(frontendTLSALPNProtocolsAnnotation),
					rawALPN,
					"ssl-alpn must include at least one non-empty protocol",
				))
				continue
			}
			if len(policy.ALPNProtocols) > maxFrontendTLSALPNProtocols {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(frontendTLSALPNProtocolsAnnotation),
					rawALPN,
					"ssl-alpn must contain no more than 16 protocols",
				))
				continue
			}
		}

		// Agentgateway FrontendTLS currently validates that handshakeTimeout is present.
		// When only ALPN is configured, project the documented default handshake timeout.
		if policy.HandshakeTimeout == nil && len(policy.ALPNProtocols) > 0 {
			policy.HandshakeTimeout = &metav1.Duration{Duration: defaultTLSHandshakeTimeout}
		}

		if policy.HandshakeTimeout == nil && len(policy.ALPNProtocols) == 0 {
			continue
		}

		key := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
		perIngress[key] = policy
	}

	if len(perIngress) == 0 {
		return errs
	}

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		routeKey := types.NamespacedName{
			Namespace: rg.Namespace,
			Name:      common.RouteName(rg.Name, rg.Host),
		}

		httpCtx, ok := ir.HTTPRoutes[routeKey]
		if !ok {
			continue
		}

		for ruleIdx, backendSources := range httpCtx.RuleBackendSources {
			for backendIdx, src := range backendSources {
				if src.Ingress == nil {
					continue
				}

				ingKey := types.NamespacedName{Namespace: src.Ingress.Namespace, Name: src.Ingress.Name}
				frontendTLS := perIngress[ingKey]
				if frontendTLS == nil {
					continue
				}

				if httpCtx.ProviderSpecificIR.IngressNginx == nil {
					httpCtx.ProviderSpecificIR.IngressNginx = &providerir.IngressNginxHTTPRouteIR{Policies: map[string]providerir.Policy{}}
				} else if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
					httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]providerir.Policy{}
				}

				existing := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if existing.FrontendTLS == nil {
					existing.FrontendTLS = &providerir.IngressNginxFrontendTLSPolicy{}
				}
				if existing.FrontendTLS.HandshakeTimeout == nil {
					existing.FrontendTLS.HandshakeTimeout = frontendTLS.HandshakeTimeout
				}
				if len(existing.FrontendTLS.ALPNProtocols) == 0 && len(frontendTLS.ALPNProtocols) > 0 {
					existing.FrontendTLS.ALPNProtocols = append([]string(nil), frontendTLS.ALPNProtocols...)
				}

				existing = existing.AddRuleBackendSources([]providerir.PolicyIndex{{Rule: ruleIdx, Backend: backendIdx}})
				httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = existing
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}
