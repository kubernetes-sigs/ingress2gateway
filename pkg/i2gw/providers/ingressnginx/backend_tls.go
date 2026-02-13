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
				
				
				primaryIngress := getNonCanaryIngress(backendSources)
				if primaryIngress == nil {
					continue
				}

				if backendIdx >= len(rule.BackendRefs) {
					continue
				}
				backendRef := rule.BackendRefs[backendIdx]

				if backendRef.Kind != nil && *backendRef.Kind != "Service" {
					continue
				}
				if backendRef.Group != nil && *backendRef.Group != "" && *backendRef.Group != "core" {
					continue
				}

				backendProtocol := primaryIngress.Annotations[BackendProtocolAnnotation]
				proxySSLVerify := primaryIngress.Annotations[ProxySSLVerifyAnnotation]
				proxySSLSecret := primaryIngress.Annotations[ProxySSLSecretAnnotation]
				proxySSLName := primaryIngress.Annotations[ProxySSLNameAnnotation]
				proxySSLServerName := primaryIngress.Annotations[ProxySSLServerNameAnnotation]
				proxySSLVerifyDepth := primaryIngress.Annotations[ProxySSLVerifyDepthAnnotation]
				proxySSLProtocols := primaryIngress.Annotations[ProxySSLProtocolsAnnotation]

				if !(backendProtocol == "HTTPS" || backendProtocol == "GRPCS") {
					continue
				}

				if proxySSLVerifyDepth != "" {
					notify(notifications.WarningNotification,
						fmt.Sprintf("Ingress %s/%s specifies %s. Gateway API v1 BackendTLSPolicy does not support configuring verification depth.",
							primaryIngress.Namespace, primaryIngress.Name, ProxySSLVerifyDepthAnnotation),
						primaryIngress,
					)
				}
				if proxySSLProtocols != "" {
					notify(notifications.WarningNotification,
						fmt.Sprintf("Ingress %s/%s specifies %s. Gateway API v1 BackendTLSPolicy does not support configuring specific TLS protocols.",
							primaryIngress.Namespace, primaryIngress.Name, ProxySSLProtocolsAnnotation),
						primaryIngress,
					)
				}

				// Strict Validation Rules to emit a policy
				var validationErrors []string

				if proxySSLVerify != "on" {
					validationErrors = append(validationErrors, fmt.Sprintf("%s must be strictly 'on'", ProxySSLVerifyAnnotation))
				}
				if proxySSLSecret == "" {
					validationErrors = append(validationErrors, fmt.Sprintf("%s must be provided with a trusted CA certificate", ProxySSLSecretAnnotation))
				}
				if proxySSLServerName != "on" { // Default is off in nginx, so must be explicitly turned on for Gateway API compatibility
					validationErrors = append(validationErrors, fmt.Sprintf("%s must be strictly 'on' (SNI is required)", ProxySSLServerNameAnnotation))
				}
				if proxySSLName == "" {
					validationErrors = append(validationErrors, fmt.Sprintf("%s must be explicitly provided (defaulting to upstream_balancer is invalid)", ProxySSLNameAnnotation))
				}

				if len(validationErrors) > 0 {
					notify(notifications.ErrorNotification,
						fmt.Sprintf("Ingress %s/%s requested backend TLS but failed strict validation requirements to emit a BackendTLSPolicy: %s",
							primaryIngress.Namespace, primaryIngress.Name, strings.Join(validationErrors, ", ")),
						primaryIngress,
					)
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
					policy = *existingPolicy.DeepCopy()
				} else {
					policy = common.CreateBackendTLSPolicy(namespace, policyName, serviceName)
				}

				// We know proxySSLName is not empty due to strict validation above
				policy.Spec.Validation.Hostname = gatewayv1.PreciseHostname(proxySSLName)

				// Handle CA Certificates
				secretName := proxySSLSecret
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

				// We know proxySSLVerify is "on" and proxySSLSecret is not empty due to strict validation above.
				policy.Spec.Validation.CACertificateRefs = []gatewayv1.LocalObjectReference{{
					Group: "",
					Kind:  "Secret",
					Name:  gatewayv1.ObjectName(secretName),
				}}
				policy.Spec.Validation.WellKnownCACertificates = nil

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
			}
		}
	}

	return errList
}
