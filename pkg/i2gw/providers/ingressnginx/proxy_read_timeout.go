/*
Copyright 2024 The Kubernetes Authors.

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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const nginxProxyReadTimeoutAnnotation = "nginx.ingress.kubernetes.io/proxy-read-timeout"

// proxyReadTimeoutFeature parses the "nginx.ingress.kubernetes.io/proxy-read-timeout"
// annotation from Ingresses and projects it into the ingress-nginx provider-specific IR.
//
// Semantics:
//   - Accepts values like "30s", "1m", etc. (time.ParseDuration).
//   - Also accepts bare numbers like "60", interpreted as seconds.
//   - Stored on intermediate.Policy.ProxyReadTimeout.
func proxyReadTimeoutFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	// Per-ingress parsed durations.
	perIngress := map[string]*metav1.Duration{}

	for _, ing := range ingresses {
		anns := ing.GetAnnotations()
		if anns == nil {
			continue
		}

		raw := strings.TrimSpace(anns[nginxProxyReadTimeoutAnnotation])
		if raw == "" {
			continue
		}

		d, err := parseProxyReadTimeoutDuration(raw)
		if err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(nginxProxyReadTimeoutAnnotation),
				raw,
				fmt.Sprintf("failed to parse proxy-read-timeout: %v", err),
			))
			continue
		}

		perIngress[ing.Name] = d
	}

	if len(perIngress) == 0 {
		return errs
	}

	// Map to HTTPRoutes using RuleBackendSources.
	for routeKey, httpCtx := range ir.HTTPRoutes {
		if httpCtx.ProviderSpecificIR.IngressNginx == nil {
			httpCtx.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
				Policies: map[string]intermediate.Policy{},
			}
		}
		if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
			httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
		}

		// Group PolicyIndex by ingress name.
		srcByIngress := map[string][]intermediate.PolicyIndex{}
		for ruleIdx, perRule := range httpCtx.RuleBackendSources {
			for backendIdx, src := range perRule {
				if src.Ingress == nil {
					continue
				}
				name := src.Ingress.Name
				srcByIngress[name] = append(srcByIngress[name],
					intermediate.PolicyIndex{Rule: ruleIdx, Backend: backendIdx},
				)
			}
		}

		for ingressName, idxs := range srcByIngress {
			d, ok := perIngress[ingressName]
			if !ok {
				continue
			}

			existing := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingressName]
			existing.ProxyReadTimeout = d
			existing.AddRuleBackendSources(idxs)
			httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingressName] = existing
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}

// parseProxyReadTimeoutDuration accepts either:
//   - standard Go duration strings ("30s", "1m", "1m30s"), or
//   - bare integer seconds like "60".
func parseProxyReadTimeoutDuration(raw string) (*metav1.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty duration")
	}

	// First, try full duration syntax.
	if d, err := time.ParseDuration(raw); err == nil {
		return &metav1.Duration{Duration: d}, nil
	}

	// Fallback: treat it as plain seconds.
	sec, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid duration %q: %w", raw, err)
	}

	d := time.Duration(sec) * time.Second
	return &metav1.Duration{Duration: d}, nil
}
