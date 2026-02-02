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
	"reflect"
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

			for backendIdx := range backendSources {
				
				// Identify the "main" ingress for this backend occurrence to check annotations.
				// We care about the annotations on the non-canary ingress (if multiple sources exist for same backend).
				// We actually need the ingress that *owns* this backend ref source.
				// Wait, backendSources is a list of BackendSource. Each BackendSource corresponds to *one* Ingress backend.
				// But we are iterating over them. 
				// Actually, `backendSources` is []BackendSource.
				// A single BackendRef in the HTTPRoute might have come from multiple Ingress backends (e.g. merge).
				// But usually 1:1 or N:1.
				// The outer loop iterates `backendSources` which effectively corresponds to `rule.BackendRefs[backendIdx]`.
				// Wait, no. `ruleBackendSources` is `[][]BackendSource`.
				// `backendSources` is `[]BackendSource` corresponding to `rule.BackendRefs[backendIdx]`.
				// So ALL these sources map to the SAME backend ref.
				
				// We want to use the annotations from the "primary" (non-canary) ingress to determine Policy.
				primaryIngress := getNonCanaryIngress(backendSources)
				if primaryIngress == nil {
					// No valid ingress source found (unlikely if loop ran).
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

				// Check for backend TLS annotations on the primary ingress
				// "nginx.ingress.kubernetes.io/backend-protocol" is the primary trigger for enabling TLS
				// Values: uppercase "HTTPS" or "GRPCS".
				backendProtocol := primaryIngress.Annotations[BackendProtocolAnnotation]
				proxySSLVerify := primaryIngress.Annotations[ProxySSLVerifyAnnotation]
				proxySSLSecret := primaryIngress.Annotations[ProxySSLSecretAnnotation]
				proxySSLName := primaryIngress.Annotations[ProxySSLNameAnnotation]
				proxySSLServerName := primaryIngress.Annotations[ProxySSLServerNameAnnotation]


				// If not HTTPS/GRPCS, skip.
				if !(backendProtocol == "HTTPS" || backendProtocol == "GRPCS") {
					continue
				}
				
				// Gateway API requires SNI for BackendTLSPolicy usually.
				// If user explicitly turned it off, we warn.
				if proxySSLServerName == "off" {
					notify(notifications.WarningNotification,
						fmt.Sprintf("Ingress %s/%s has %s set to 'off'. Gateway API BackendTLSPolicy typically enforces SNI via the Hostname field. A Hostname will still be generated.",
							primaryIngress.Namespace, primaryIngress.Name, ProxySSLServerNameAnnotation),
						primaryIngress,
					)
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
					// because Gateway API v1 BackendTLSPolicy requires Hostname to be non-empty.
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

							if secretNamespace != namespace {
								notify(notifications.ErrorNotification,
									fmt.Sprintf("Ingress %s/%s specifies backend TLS secret %s in a different namespace. BackendTLSPolicy only supports local Secrets. Policy will not be generated.",
										primaryIngress.Namespace, primaryIngress.Name, proxySSLSecret),
									primaryIngress,
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
						// Explicitly clear WellKnown for custom CA
						policy.Spec.Validation.WellKnownCACertificates = nil
					} else {
						// Verify on, no secret provided -> Implies system trust.
						system := gatewayv1.WellKnownCACertificatesSystem
						policy.Spec.Validation.WellKnownCACertificates = &system
						policy.Spec.Validation.CACertificateRefs = nil
					}
				} else {
					// Verify off.
					// Gateway API v1 BackendTLSPolicy requires at least one validation method, so we default to System.
					// We warn the user that validation is being enabled (as "off" is not strictly supported).
					notify(notifications.WarningNotification,
						fmt.Sprintf("Ingress %s/%s has %s set to 'off' (or default off). Gateway API BackendTLSPolicy requires validation. defaulting to System Trust.",
							primaryIngress.Namespace, primaryIngress.Name, ProxySSLVerifyAnnotation),
						primaryIngress,
					)
					
					system := gatewayv1.WellKnownCACertificatesSystem
					policy.Spec.Validation.WellKnownCACertificates = &system
					policy.Spec.Validation.CACertificateRefs = nil
				}

				if exists {
					// Check for conflict using DeepEqual
					if !reflect.DeepEqual(policy.Spec.Validation, existingPolicy.Spec.Validation) {
						notify(notifications.WarningNotification,
							fmt.Sprintf("Conflict detected for BackendTLSPolicy %s. Ingress %s/%s defines different TLS settings than a previously processed Ingress. Keeping the first one.",
								policyName, primaryIngress.Namespace, primaryIngress.Name),
							primaryIngress,
						)
					}
					// If exists, we keep the existing one (first wins strategy)
					continue
				}

				ir.BackendTLSPolicies[policyKey] = policy
				
				// Since we processed the backendRef (which combines all sources), we can break the inner source loop?
				// Actually, we extracted `primaryIngress` from `backendSources` which IS the collection of sources for this backendRef.
				// We just need to process this *once* for the BackendRef.
				// So yes, we should break here to avoid reprocessing the same BackendRef for every minor source (though they should yield same result).
				break
			}
		}
	}

	return errList
}
