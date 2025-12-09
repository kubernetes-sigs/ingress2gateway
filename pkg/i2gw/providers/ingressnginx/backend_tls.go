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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	nginxProxySSLSecret = "nginx.ingress.kubernetes.io/proxy-ssl-secret"
	nginxProxySSLVerify = "nginx.ingress.kubernetes.io/proxy-ssl-verify"
	nginxProxySSLName   = "nginx.ingress.kubernetes.io/proxy-ssl-name"
	// nginxProxySSLServerName = "nginx.ingress.kubernetes.io/proxy-ssl-server-name" // Not relevant to Gateway API
)

// backendTLSFeature parses backend TLS annotations and stores them in the IR Policy.
// It also creates BackendTLSPolicy resources in the IR for each unique service that
// requires backend TLS configuration.
//
// Semantics:
//   - proxy-ssl-secret: Specifies a Secret with tls.crt, tls.key, and ca.crt in PEM format.
//     Format: "namespace/secretName"
//   - proxy-ssl-verify: Enables or disables verification of the proxied HTTPS server certificate.
//     Values: "on" or "off" (default: "off")
//   - proxy-ssl-name: Overrides the server name used to verify the certificate and passed via SNI.
//     In Gateway API, this maps to Validation.Hostname, which enables SNI automatically.
//   - proxy-ssl-server-name: Not handled separately. In Gateway API, SNI is enabled when Hostname is set.
func backendTLSFeature(
	ingresses []networkingv1.Ingress,
	servicePorts map[types.NamespacedName]map[string]int32,
	ir *intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	// Per-Ingress parsed backend TLS policy.
	perIngress := map[types.NamespacedName]*intermediate.BackendTLSPolicy{}

	// Track which services need BackendTLSPolicy resources.
	// Key: service namespaced name, Value: backend TLS policy data
	serviceBackendTLS := map[types.NamespacedName]*backendTLSPolicyData{}

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

		// Collect services that need BackendTLSPolicy from this ingress
		collectServicesFromIngress(ing, policy, serviceBackendTLS)
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

	// Create BackendTLSPolicy resources for each service
	for svcKey, tlsData := range serviceBackendTLS {
		policyName := generateBackendTLSPolicyName(svcKey)
		policy := createBackendTLSPolicy(svcKey.Namespace, policyName, svcKey.Name, tlsData)

		policyKey := types.NamespacedName{
			Namespace: svcKey.Namespace,
			Name:      policyName,
		}
		ir.BackendTLSPolicies[policyKey] = policy
	}

	return errs
}

// backendTLSPolicyData holds the data needed to create a BackendTLSPolicy.
type backendTLSPolicyData struct {
	// SecretNamespace and SecretName are parsed from the "namespace/secretName" format
	SecretNamespace string
	SecretName      string
	// Verify enables certificate verification
	Verify bool
	// Hostname is the server name for SNI and certificate verification.
	// In Gateway API, setting Hostname enables SNI automatically.
	Hostname string
}

// collectServicesFromIngress collects all services referenced by an ingress that need BackendTLSPolicy.
func collectServicesFromIngress(ing *networkingv1.Ingress, policy *intermediate.BackendTLSPolicy, serviceBackendTLS map[types.NamespacedName]*backendTLSPolicyData) {
	// Parse secret name (format: "namespace/secretName")
	secretNamespace := ing.Namespace
	secretName := policy.SecretName
	if parts := strings.SplitN(policy.SecretName, "/", 2); len(parts) == 2 {
		secretNamespace = parts[0]
		secretName = parts[1]
	}

	tlsData := &backendTLSPolicyData{
		SecretNamespace: secretNamespace,
		SecretName:      secretName,
		Verify:          policy.Verify,
		Hostname:        policy.Hostname,
	}

	// Collect services from default backend
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		svcKey := types.NamespacedName{
			Namespace: ing.Namespace,
			Name:      ing.Spec.DefaultBackend.Service.Name,
		}
		serviceBackendTLS[svcKey] = tlsData
	}

	// Collect services from rules
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				svcKey := types.NamespacedName{
					Namespace: ing.Namespace,
					Name:      path.Backend.Service.Name,
				}
				serviceBackendTLS[svcKey] = tlsData
			}
		}
	}
}

// generateBackendTLSPolicyName generates a name for the BackendTLSPolicy based on the service name.
func generateBackendTLSPolicyName(svcKey types.NamespacedName) string {
	return svcKey.Name + "-backend-tls"
}

// createBackendTLSPolicy creates a BackendTLSPolicy resource.
func createBackendTLSPolicy(namespace, policyName, serviceName string, tlsData *backendTLSPolicyData) gatewayv1.BackendTLSPolicy {
	policy := gatewayv1.BackendTLSPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1.GroupVersion.String(),
			Kind:       "BackendTLSPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
		Spec: gatewayv1.BackendTLSPolicySpec{
			TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
				{
					LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
						Group: "", // Core group
						Kind:  "Service",
						Name:  gatewayv1.ObjectName(serviceName),
					},
				},
			},
			Validation: gatewayv1.BackendTLSPolicyValidation{},
		},
	}

	// Set CA certificate reference if secret is specified and verification is enabled
	// Note: In Gateway API, the presence of CACertificateRefs implies certificate verification.
	// If proxy-ssl-verify is "off", we don't set CACertificateRefs to match nginx behavior.
	if tlsData.SecretName != "" && tlsData.Verify {
		policy.Spec.Validation.CACertificateRefs = []gatewayv1.LocalObjectReference{
			{
				Group: "", // Core group
				Kind:  "Secret",
				Name:  gatewayv1.ObjectName(tlsData.SecretName),
			},
		}
	}

	// Set hostname if proxy-ssl-name is specified.
	// In Gateway API, setting Validation.Hostname enables SNI automatically.
	// The same hostname value is used for both SNI and certificate validation.
	if tlsData.Hostname != "" {
		policy.Spec.Validation.Hostname = gatewayv1.PreciseHostname(tlsData.Hostname)
	}

	// Note: Gateway API BackendTLSPolicy limitations:
	// - Client certificates (tls.crt, tls.key from the secret) are not directly supported.
	//   These are implementation-specific and may need to be configured separately.
	// - proxy-ssl-verify is mapped to the presence/absence of CACertificateRefs.
	//   If verify is "off", CACertificateRefs is not set.
	// - proxy-ssl-name maps to Validation.Hostname, which enables SNI automatically.
	//   There is no separate "enable SNI" boolean in Gateway API.
	// - proxy-ssl-server-name is not handled separately since SNI is enabled when Hostname is set.

	return policy
}
