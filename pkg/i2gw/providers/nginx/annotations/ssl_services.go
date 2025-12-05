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

package annotations

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// SSLServicesFeature processes nginx.org/ssl-services annotation
func SSLServicesFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *provider_intermediate.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	for _, ingress := range ingresses {
		if sslServices, exists := ingress.Annotations[nginxSSLServicesAnnotation]; exists && sslServices != "" {
			errs = append(errs, processSSLServicesAnnotation(ingress, sslServices, ir)...)
		}
	}

	return errs
}

// processSSLServicesAnnotation configures HTTPS backend protocol using BackendTLSPolicy
//
//nolint:unparam // ErrorList return type maintained for consistency
func processSSLServicesAnnotation(ingress networkingv1.Ingress, sslServices string, ir *provider_intermediate.ProviderIR) field.ErrorList {
	var errs field.ErrorList //nolint:unparam // ErrorList return type maintained for consistency

	services := splitAndTrimCommaList(sslServices)
	sslServiceSet := make(map[string]struct{})
	for _, service := range services {
		sslServiceSet[service] = struct{}{}
	}

	if ir.BackendTLSPolicies == nil {
		ir.BackendTLSPolicies = make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy)
	}
	for serviceName := range sslServiceSet {
		policyName := BackendTLSPolicyName(ingress.Name, serviceName)
		policy := common.CreateBackendTLSPolicy(ingress.Namespace, policyName, serviceName)
		policyKey := types.NamespacedName{
			Namespace: ingress.Namespace,
			Name:      policyName,
		}

		ir.BackendTLSPolicies[policyKey] = policy
	}

	// Add warning about manual certificate configuration
	if len(sslServiceSet) > 0 {
		message := "nginx.org/ssl-services: " + BackendTLSPolicyKind + " created but requires manual configuration. You must set the 'validation.hostname' field to match your backend service's TLS certificate hostname, and configure appropriate CA certificates or certificateRefs for TLS verification."
		notify(notifications.WarningNotification, message, &ingress)
	}

	return errs
}

// BackendTLSPolicyName returns the generated name for a BackendTLSPolicy using NGINX naming convention
func BackendTLSPolicyName(ingressName, serviceName string) string {
	return fmt.Sprintf("%s-%s-backend-tls", ingressName, serviceName)
}
