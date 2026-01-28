/*
Copyright 2025 The Kubernetes Authors.

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
	"strings"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

func backendTLSFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errList field.ErrorList

	if ir.BackendTLSPolicies == nil {
		ir.BackendTLSPolicies = make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy)
	}

	for _, httpRouteContext := range ir.HTTPRoutes {
		for ruleIdx, backendSources := range httpRouteContext.RuleBackendSources {
			if ruleIdx >= len(httpRouteContext.HTTPRoute.Spec.Rules) {
				continue
			}
			rule := httpRouteContext.HTTPRoute.Spec.Rules[ruleIdx]

			for backendIdx, source := range backendSources {
				if source.Ingress == nil {
					continue
				}

				// Skip canary ingresses for BackendTLSPolicy generation
				if source.Ingress.Annotations[CanaryAnnotation] == "true" {
					continue
				}

				// Check for backend TLS annotations
				// "nginx.ingress.kubernetes.io/backend-protocol" is the primary trigger for enabling TLS
				// Values: uppercase "HTTPS" or "GRPCS".
				backendProtocol := source.Ingress.Annotations[BackendProtocolAnnotation]
				proxySSLVerify := source.Ingress.Annotations[ProxySSLVerifyAnnotation]
				proxySSLSecret := source.Ingress.Annotations[ProxySSLSecretAnnotation]
				proxySSLName := source.Ingress.Annotations[ProxySSLNameAnnotation]

				// If not HTTPS/GRPCS, skip.
				// Nginx only speaks TLS to backend if backend-protocol is set to HTTPS/GRPCS.
				if !( backendProtocol == "HTTPS" || backendProtocol == "GRPCS") {
					continue
				}

				if backendIdx >= len(rule.BackendRefs) {
					continue
				}
				backendRef := rule.BackendRefs[backendIdx]

				// We can only apply BackendTLSPolicy to Services
				if backendRef.Kind != nil && *backendRef.Kind != "Service" {
					continue
				}
				if backendRef.Group != nil && *backendRef.Group != "" && *backendRef.Group != "core" {
					continue
				}

				serviceName := string(backendRef.Name)
				namespace := httpRouteContext.HTTPRoute.Namespace
				if backendRef.Namespace != nil {
					namespace = string(*backendRef.Namespace)
				}

				policyName := fmt.Sprintf("%s-backend-tls", serviceName)
				policyKey := types.NamespacedName{Namespace: namespace, Name: policyName}

				// Check if we already created a policy for this service
				existingPolicy, exists := ir.BackendTLSPolicies[policyKey]
				var policy gatewayv1.BackendTLSPolicy
				if exists {
					policy = existingPolicy
				} else {
					policy = common.CreateBackendTLSPolicy(namespace, policyName, serviceName)
				}

				// Map proxy-ssl-name to Hostname
				if proxySSLName != "" {
					policy.Spec.Validation.Hostname = gatewayv1.PreciseHostname(proxySSLName)
				} else {
					// If proxy-ssl-name is not set, we default to the Service DNS name
					// because Gateway API v1 BackendTLSPolicy requires Hostname to be non-empty (no omitempty tag).
					// This aligns with typical cluster-internal TLS where the cert often matches the service DNS.
					dnsName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
					policy.Spec.Validation.Hostname = gatewayv1.PreciseHostname(dnsName)
				}

				// Handle CA Certificates
				secretName := proxySSLSecret
				if secretName != "" {
					if strings.Contains(secretName, "/") {
						parts := strings.SplitN(secretName, "/", 2)
						if len(parts) == 2 {
							secretNamespace := parts[0]
							secretName = parts[1]

							// Gateway API requires BackendTLSPolicy CA Reference to be in the same namespace
							// (unless using ReferenceGrants, but we are generating LocalObjectReference here which implies same namespace).
							// If Ingress specifies a Secret in a different namespace, we cannot support it directly without ReferenceGrant.
							// For now, we error/warn and skip.
							if secretNamespace != namespace {
								notify(notifications.ErrorNotification,
									fmt.Sprintf("Ingress %s/%s specifies backend TLS secret %s in a different namespace. BackendTLSPolicy only supports local Secrets. Policy will not be generated.",
										source.Ingress.Namespace, source.Ingress.Name, proxySSLSecret),
									source.Ingress,
								)
								continue
							}
						}
					}
				}

				if proxySSLVerify == "on" {
					if proxySSLSecret != "" {
						policy.Spec.Validation.CACertificateRefs = []gatewayv1.LocalObjectReference{{
							Group: "",
							Kind:  "Secret",
							Name:  gatewayv1.ObjectName(secretName),
						}}
					} else {
						// Verify on, no secret provided -> Implies system trust.
						system := gatewayv1.WellKnownCACertificatesSystem
						policy.Spec.Validation.WellKnownCACertificates = &system
					}
				} else {
					// Verify off.
					// We treat this as "System Trust" because Gateway API v1 BackendTLSPolicy
					// requires at least one validation method.
					system := gatewayv1.WellKnownCACertificatesSystem
					policy.Spec.Validation.WellKnownCACertificates = &system
				}

				if exists {
					// Compare significant fields to see if there is a conflict
					// We only check CA refs and Hostname for now as those are what we set
					currentCA := policy.Spec.Validation.CACertificateRefs
					existingCA := existingPolicy.Spec.Validation.CACertificateRefs
					currentHostname := policy.Spec.Validation.Hostname
					existingHostname := existingPolicy.Spec.Validation.Hostname

					// Simple comparison
					caAuthMismatch := false
					if len(currentCA) != len(existingCA) {
						caAuthMismatch = true
					} else if len(currentCA) > 0 {
						if currentCA[0].Name != existingCA[0].Name {
							caAuthMismatch = true
						}
					}

					hostnameMismatch := currentHostname != existingHostname

					if caAuthMismatch || hostnameMismatch {
						notify(notifications.WarningNotification,
							fmt.Sprintf("Conflict detected for BackendTLSPolicy %s. Ingress %s/%s defines different TLS settings than a previously processed Ingress. Keeping the first one.",
								policyName, source.Ingress.Namespace, source.Ingress.Name),
							source.Ingress,
						)
					}
					// If exists, we keep the existing one (first wins strategy)
					continue
				}

				ir.BackendTLSPolicies[policyKey] = policy
			}
		}
	}

	return errList
}
