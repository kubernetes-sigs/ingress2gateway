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
	"strconv"
	"strings"
	"time"

	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/providers/common"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	nginxAffinityAnnotation              = "nginx.ingress.kubernetes.io/affinity"
	nginxSessionCookiePathAnnotation     = "nginx.ingress.kubernetes.io/session-cookie-path"
	nginxSessionCookieDomainAnnotation   = "nginx.ingress.kubernetes.io/session-cookie-domain"
	nginxSessionCookieSameSiteAnnotation = "nginx.ingress.kubernetes.io/session-cookie-samesite"
	nginxSessionCookieExpiresAnnotation  = "nginx.ingress.kubernetes.io/session-cookie-expires"
	nginxSessionCookieMaxAgeAnnotation   = "nginx.ingress.kubernetes.io/session-cookie-max-age"
	nginxSessionCookieSecureAnnotation   = "nginx.ingress.kubernetes.io/session-cookie-secure"
	nginxSessionCookieNameAnnotation     = "nginx.ingress.kubernetes.io/session-cookie-name"
)

// sessionAffinityFeature parses session affinity annotations and stores them in the IR Policy.
//
// Semantics:
//   - The affinity annotation enables session affinity (only "cookie" type is supported).
//   - Session cookie annotations configure the cookie properties.
//   - We normalize cookie expires to metav1.Duration and attach it per-Ingress, then map to
//     specific (rule, backend) pairs via RuleBackendSources.
func sessionAffinityFeature(
	ingresses []networkingv1.Ingress,
	_ map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	// Per-Ingress parsed session affinity policy.
	perIngress := map[types.NamespacedName]*intermediate.SessionAffinityPolicy{}

	for i := range ingresses {
		ing := &ingresses[i]
		anns := ing.Annotations
		if anns == nil {
			continue
		}

		// Check if affinity is enabled
		affinityType := strings.TrimSpace(anns[nginxAffinityAnnotation])
		if affinityType == "" || affinityType != "cookie" {
			// Only cookie affinity is supported
			continue
		}

		key := types.NamespacedName{
			Namespace: ing.Namespace,
			Name:      ing.Name,
		}

		policy := &intermediate.SessionAffinityPolicy{
			CookieName: "INGRESSCOOKIE", // Default cookie name used by NGINX
		}

		// Parse cookie name (if specified)
		if cookieName := strings.TrimSpace(anns[nginxSessionCookieNameAnnotation]); cookieName != "" {
			policy.CookieName = cookieName
		}

		// Parse cookie path
		if cookiePath := strings.TrimSpace(anns[nginxSessionCookiePathAnnotation]); cookiePath != "" {
			policy.CookiePath = cookiePath
		}

		// Parse cookie domain
		if cookieDomain := strings.TrimSpace(anns[nginxSessionCookieDomainAnnotation]); cookieDomain != "" {
			policy.CookieDomain = cookieDomain
		}

		// Parse cookie SameSite
		if cookieSameSite := strings.TrimSpace(anns[nginxSessionCookieSameSiteAnnotation]); cookieSameSite != "" {
			// Validate SameSite values
			if cookieSameSite == "None" || cookieSameSite == "Lax" || cookieSameSite == "Strict" {
				policy.CookieSameSite = cookieSameSite
			} else {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(nginxSessionCookieSameSiteAnnotation),
					cookieSameSite,
					"session-cookie-samesite must be one of: None, Lax, Strict",
				))
				continue
			}
		}

		// Parse cookie expires (TTL) - max-age takes precedence over expires
		parseDuration := func(annotationValue string) (*metav1.Duration, error) {
			// value is in seconds, e.g. "3600"
			d, err := time.ParseDuration(annotationValue + "s")
			if err != nil {
				return nil, err
			}
			return &metav1.Duration{Duration: d}, nil
		}

		// Parse session-cookie-max-age first (takes precedence)
		if cookieMaxAgeRaw := strings.TrimSpace(anns[nginxSessionCookieMaxAgeAnnotation]); cookieMaxAgeRaw != "" {
			duration, err := parseDuration(cookieMaxAgeRaw)
			if err != nil {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(nginxSessionCookieMaxAgeAnnotation),
					cookieMaxAgeRaw,
					"failed to parse session-cookie-max-age",
				))
				continue
			}
			policy.CookieExpires = duration
		} else if cookieExpiresRaw := strings.TrimSpace(anns[nginxSessionCookieExpiresAnnotation]); cookieExpiresRaw != "" {
			// Only parse expires if max-age is not set
			duration, err := parseDuration(cookieExpiresRaw)
			if err != nil {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(nginxSessionCookieExpiresAnnotation),
					cookieExpiresRaw,
					"failed to parse session-cookie-expires",
				))
				continue
			}
			policy.CookieExpires = duration
		}

		// Parse cookie secure
		if cookieSecureRaw := strings.TrimSpace(anns[nginxSessionCookieSecureAnnotation]); cookieSecureRaw != "" {
			secure, err := strconv.ParseBool(cookieSecureRaw)
			if err != nil {
				errs = append(errs, field.Invalid(
					field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(nginxSessionCookieSecureAnnotation),
					cookieSecureRaw,
					"failed to parse session-cookie-secure (must be true or false)",
				))
				continue
			}
			policy.CookieSecure = &secure
		}

		perIngress[key] = policy
	}

	if len(perIngress) == 0 {
		return errs
	}

	// Map per-Ingress session affinity policy onto HTTPRoute policies using RuleBackendSources.
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

		if httpCtx.ProviderSpecificIR.IngressNginx == nil {
			httpCtx.ProviderSpecificIR.IngressNginx = &intermediate.IngressNginxHTTPRouteIR{
				Policies: map[string]intermediate.Policy{},
			}
		}
		if httpCtx.ProviderSpecificIR.IngressNginx.Policies == nil {
			httpCtx.ProviderSpecificIR.IngressNginx.Policies = map[string]intermediate.Policy{}
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

				sessionAffinity := perIngress[ingKey]
				if sessionAffinity == nil {
					continue
				}

				p := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if p.SessionAffinity == nil {
					// Deep copy the policy to avoid sharing references
					sessionAffinityCopy := *sessionAffinity
					p.SessionAffinity = &sessionAffinityCopy
				}

				// Dedupe (rule, backend) pairs.
				p = p.AddRuleBackendSources([]intermediate.PolicyIndex{
					{
						Rule:    ruleIdx,
						Backend: backendIdx,
					},
				})

				httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name] = p
			}
		}

		ir.HTTPRoutes[routeKey] = httpCtx
	}

	return errs
}
