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

				// Check for backend TLS annotations
				// "nginx.ingress.kubernetes.io/backend-protocol" is the primary trigger for enabling TLS
				// Values: uppercase "HTTPS" or "GRPCS".
				backendProtocol := source.Ingress.Annotations[BackendProtocolAnnotation]
				proxySSLVerify := source.Ingress.Annotations[ProxySSLVerifyAnnotation]
				proxySSLSecret := source.Ingress.Annotations[ProxySSLSecretAnnotation]
				proxySSLName := source.Ingress.Annotations[ProxySSLNameAnnotation]

				isHTTPS := backendProtocol == "HTTPS" || backendProtocol == "GRPCS"

				// If not HTTPS/GRPCS, skip.
				// Nginx only speaks TLS to backend if backend-protocol is set to HTTPS/GRPCS.
				if !isHTTPS {
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
				policy, exists := ir.BackendTLSPolicies[policyKey]
				if !exists {
					policy = common.CreateBackendTLSPolicy(namespace, policyName, serviceName)
				}

				// Map proxy-ssl-name to Hostname
				if proxySSLName != "" {
					policy.Spec.Validation.Hostname = gatewayv1.PreciseHostname(proxySSLName)
				}

				// Handle CA Certificates
				secretName := proxySSLSecret
				if strings.Contains(secretName, "/") {
					parts := strings.SplitN(secretName, "/", 2)
					if len(parts) == 2 {
						secretName = parts[1]
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
						// Gateway API: "If the list is empty, the backend's certificate will be verified against the system trust store."
						policy.Spec.Validation.CACertificateRefs = []gatewayv1.LocalObjectReference{}
					}
				} else {
					// Verify off.
					// We treat this as "System Trust" or "No explicit CA".
					// Note: Gateway API v1 doesn't support "InsecureSkipVerify" in standard fields.
					policy.Spec.Validation.CACertificateRefs = nil
				}

				ir.BackendTLSPolicies[policyKey] = policy
			}
		}
	}

	return errList
}
