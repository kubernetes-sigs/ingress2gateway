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
	nginxProxySSLSecret = "nginx.ingress.kubernetes.io/proxy-ssl-secret"
	nginxProxySSLVerify = "nginx.ingress.kubernetes.io/proxy-ssl-verify"
	nginxProxySSLName   = "nginx.ingress.kubernetes.io/proxy-ssl-name"
	// nginxProxySSLServerName = "nginx.ingress.kubernetes.io/proxy-ssl-server-name" // Not relevant to Gateway API
)

// backendTLSFeature parses backend TLS annotations and stores them in the IR Policy.
// The TLS configuration is then applied to kgateway BackendConfigPolicy resources
// in the kgateway emitter.
//
// Semantics:
//   - proxy-ssl-secret: Specifies a Secret with tls.crt, tls.key, and ca.crt in PEM format.
//     Format: "namespace/secretName"
//   - proxy-ssl-verify: Enables or disables verification of the proxied HTTPS server certificate.
//     Values: "on" or "off" (default: "off")
//   - proxy-ssl-name: Overrides the server name used to verify the certificate and passed via SNI.
//     In kgateway BackendConfigPolicy, this maps to TLS configuration fields.
//   - proxy-ssl-server-name: Not handled separately. SNI is enabled when hostname is set.
func backendTLSFeature(
	ingresses []networkingv1.Ingress,
	servicePorts map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	// Per-Ingress parsed backend TLS policy.
	perIngress := map[types.NamespacedName]*intermediate.BackendTLSPolicy{}

	for i := range ingresses {
		ing := &ingresses[i]
		anns := ing.Annotations
		if anns == nil {
			continue
		}

		// Check if proxy-ssl-secret is specified (required for backend TLS)
		secretName := strings.TrimSpace(anns[nginxProxySSLSecret])
		if secretName == "" {
			continue
		}

		// Validate secret name format (should be "namespace/secretName" or just "secretName")
		secretParts := strings.SplitN(secretName, "/", 2)
		if len(secretParts) > 2 {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ing.Namespace, ing.Name, "metadata", "annotations").Key(nginxProxySSLSecret),
				secretName,
				"proxy-ssl-secret must be in format 'secretName' or 'namespace/secretName'",
			))
			continue
		}

		key := types.NamespacedName{
			Namespace: ing.Namespace,
			Name:      ing.Name,
		}

		policy := &intermediate.BackendTLSPolicy{
			SecretName: secretName,
			Verify:     false, // default: off
		}

		// Parse proxy-ssl-verify (values: "on" or "off")
		if verifyRaw := strings.TrimSpace(anns[nginxProxySSLVerify]); verifyRaw != "" {
			if strings.ToLower(verifyRaw) == "on" {
				policy.Verify = true
			}
			// "off" is the default, so no action needed
		}

		// Parse proxy-ssl-name
		// This maps to both SNI and hostname validation in Gateway API.
		// In Gateway API, setting Hostname enables SNI automatically.
		if hostname := strings.TrimSpace(anns[nginxProxySSLName]); hostname != "" {
			policy.Hostname = hostname
		}

		// Note: proxy-ssl-server-name is not handled separately.
		// In Gateway API, SNI is enabled by setting the Hostname field.
		// If proxy-ssl-name is set, SNI is automatically enabled.

		perIngress[key] = policy
	}

	if len(perIngress) == 0 {
		return errs
	}

	// Map per-Ingress backend TLS policy onto HTTPRoute policies using RuleBackendSources.
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

				backendTLS := perIngress[ingKey]
				if backendTLS == nil {
					continue
				}

				p := httpCtx.ProviderSpecificIR.IngressNginx.Policies[ingKey.Name]
				if p.BackendTLS == nil {
					// Deep copy the policy to avoid sharing references
					backendTLSCopy := *backendTLS
					p.BackendTLS = &backendTLSCopy
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
