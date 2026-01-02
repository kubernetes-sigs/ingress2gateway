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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	backendProtocolAnnotation = "nginx.ingress.kubernetes.io/backend-protocol"
)

// createBackendTLSPolicies inspects ingresses for backend-protocol annotations
// and creates BackendTLSPolicies if HTTPS or GRPCS is specified.
func createBackendTLSPolicies(ingresses []networkingv1.Ingress, servicePorts map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errList field.ErrorList

	for _, rg := range ruleGroups {
		// Determine protocol for this rule group (host).
		var protocolType string

		for _, rule := range rg.Rules {
			if val, ok := rule.Ingress.Annotations[backendProtocolAnnotation]; ok {
				if val != "" {
					protocolType = val
					break
				}
			}
		}

		if protocolType == "" {
			continue
		}

		// Handle HTTPS and GRPCS (TLS Policy)
		if protocolType == "HTTPS" || protocolType == "GRPCS" {
			// We iterate the Rules in the Group to find backends
			for _, rule := range rg.Rules {
				for _, path := range rule.IngressRule.HTTP.Paths {
					backendRef, err := common.ToBackendRef(rg.Namespace, path.Backend, servicePorts, field.NewPath("backend"))
					if err != nil {
						errList = append(errList, err)
						continue
					}
					serviceName := string(backendRef.Name)
					if serviceName == "" {
						continue
					}
					policyName := fmt.Sprintf("%s-tls-policy", serviceName)
					policyKey := types.NamespacedName{Namespace: rg.Namespace, Name: policyName}

					if _, exists := ir.BackendTLSPolicies[policyKey]; !exists {
						policy := common.CreateBackendTLSPolicy(rg.Namespace, policyName, serviceName)
						ir.BackendTLSPolicies[policyKey] = policy
						notify(notifications.InfoNotification, fmt.Sprintf("Created BackendTLSPolicy %s/%s for service %s due to backend-protocol %q",
							policy.Namespace, policy.Name, serviceName, protocolType), &policy)
					}
				}
			}
		}
	}
	return errList
}
