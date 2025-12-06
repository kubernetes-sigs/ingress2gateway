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
	"strings"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	authURLAnnotation             = "nginx.ingress.kubernetes.io/auth-url"
	authResponseHeadersAnnotation = "nginx.ingress.kubernetes.io/auth-response-headers"
	authTypeAnnotation            = "nginx.ingress.kubernetes.io/auth-type"
	authSecretAnnotation          = "nginx.ingress.kubernetes.io/auth-secret"
)

// extAuthFeature extracts the "auth-url" and "auth-response-headers" annotations and
// projects them into the provider-specific IR similarly to other annotation features.
func extAuthFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {

	var errs field.ErrorList
	ingressPolicies := map[types.NamespacedName]*intermediate.Policy{}

	for i := range ingresses {
		ing := &ingresses[i]
		authURLRaw := strings.TrimSpace(ing.Annotations[authURLAnnotation])
		authResponseHeadersRaw := strings.TrimSpace(ing.Annotations[authResponseHeadersAnnotation])

		// Skip if neither annotation is present
		if authURLRaw == "" && authResponseHeadersRaw == "" {
			continue
		}

		key := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
		pol := ingressPolicies[key]
		if pol == nil {
			pol = &intermediate.Policy{}
			ingressPolicies[key] = pol
		}

		if pol.ExtAuth == nil {
			pol.ExtAuth = &intermediate.ExtAuthPolicy{}
		}

		if authURLRaw != "" {
			pol.ExtAuth.AuthURL = authURLRaw
		}

		if authResponseHeadersRaw != "" {
			var headers []string
			for _, part := range strings.Split(authResponseHeadersRaw, ",") {
				v := strings.TrimSpace(part)
				if v != "" {
					headers = append(headers, v)
				}
			}
			pol.ExtAuth.ResponseHeaders = headers
		}
	}

	if len(ingressPolicies) == 0 {
		return errs
	}

	// Map policies to HTTPRoutes (same pattern as other features)
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

				ingKey := types.NamespacedName{
					Namespace: src.Ingress.Namespace,
					Name:      src.Ingress.Name,
				}

				pol := ingressPolicies[ingKey]
				if pol == nil || pol.ExtAuth == nil {
					continue
				}

				// Ensure provider-specific IR exists
				if httpCtx.ProviderSpecificIR.IngressNginx == nil {
					httpCtx.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
						Policies: map[string]intermediate.Policy{},
					}
				} else if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
					httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
				}

				existing := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if existing.ExtAuth == nil {
					existing.ExtAuth = pol.ExtAuth
				} else {
					// Merge ExtAuth policy: preserve existing values if new ones are empty
					if pol.ExtAuth.AuthURL != "" {
						existing.ExtAuth.AuthURL = pol.ExtAuth.AuthURL
					}
					if len(pol.ExtAuth.ResponseHeaders) > 0 {
						existing.ExtAuth.ResponseHeaders = pol.ExtAuth.ResponseHeaders
					}
				}

				// Dedupe (rule, backend) pairs.
				existing = existing.AddRuleBackendSources([]intermediate.PolicyIndex{
					{Rule: ruleIdx, Backend: backendIdx},
				})

				httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = existing
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}

// basicAuthFeature extracts the "auth-type" and "auth-secret" annotations and
// projects them into the provider-specific IR similarly to other annotation features.
func basicAuthFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList
	ingressPolicies := map[types.NamespacedName]*intermediate.Policy{}

	for i := range ingresses {
		ing := &ingresses[i]
		authTypeRaw := strings.TrimSpace(ing.Annotations[authTypeAnnotation])
		authSecretRaw := strings.TrimSpace(ing.Annotations[authSecretAnnotation])

		// Only process if auth-type is "basic" and auth-secret is present
		if authTypeRaw != "basic" || authSecretRaw == "" {
			continue
		}

		key := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
		pol := ingressPolicies[key]
		if pol == nil {
			pol = &intermediate.Policy{}
			ingressPolicies[key] = pol
		}

		// Parse secret reference (format: namespace/name or just name)
		secretName := authSecretRaw
		if strings.Contains(authSecretRaw, "/") {
			parts := strings.SplitN(authSecretRaw, "/", 2)
			if len(parts) == 2 {
				// If secret is in different namespace, use just the name
				// (kgateway expects secret in same namespace as TrafficPolicy)
				secretName = parts[1]
			}
		}

		pol.BasicAuth = &intermediate.BasicAuthPolicy{
			SecretName: secretName,
		}
	}

	if len(ingressPolicies) == 0 {
		return errs
	}

	// Map policies to HTTPRoutes (same pattern as other features)
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

				ingKey := types.NamespacedName{
					Namespace: src.Ingress.Namespace,
					Name:      src.Ingress.Name,
				}

				pol := ingressPolicies[ingKey]
				if pol == nil || pol.BasicAuth == nil {
					continue
				}

				// Ensure provider-specific IR exists
				if httpCtx.ProviderSpecificIR.IngressNginx == nil {
					httpCtx.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
						Policies: map[string]intermediate.Policy{},
					}
				} else if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
					httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
				}

				existing := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if existing.BasicAuth == nil {
					existing.BasicAuth = pol.BasicAuth
				}

				// Dedupe (rule, backend) pairs.
				existing = existing.AddRuleBackendSources([]intermediate.PolicyIndex{
					{Rule: ruleIdx, Backend: backendIdx},
				})

				httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = existing
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}
