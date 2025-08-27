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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

// SSLServicesFeature processes nginx.org/ssl-services annotation
func SSLServicesFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
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
func processSSLServicesAnnotation(ingress networkingv1.Ingress, sslServices string, ir *intermediate.IR) field.ErrorList {
	var errs field.ErrorList //nolint:unparam // ErrorList return type maintained for consistency

	services := splitAndTrimCommaList(sslServices)
	sslServiceSet := make(map[string]struct{})
	for _, service := range services {
		sslServiceSet[service] = struct{}{}
	}

	if ir.BackendTLSPolicies == nil {
		ir.BackendTLSPolicies = make(map[types.NamespacedName]gatewayv1alpha3.BackendTLSPolicy)
	}
	for serviceName := range sslServiceSet {
		policyName := fmt.Sprintf("%s-%s-backend-tls", ingress.Name, serviceName)
		policyKey := types.NamespacedName{
			Namespace: ingress.Namespace,
			Name:      policyName,
		}

		policy := gatewayv1alpha3.BackendTLSPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gatewayv1alpha3.GroupVersion.String(),
				Kind:       BackendTLSPolicyKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: ingress.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "ingress2gateway",
				},
			},
			Spec: gatewayv1alpha3.BackendTLSPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: gatewayv1alpha2.LocalPolicyTargetReference{
							Group: CoreGroup,
							Kind:  ServiceKind,
							Name:  gatewayv1.ObjectName(serviceName),
						},
					},
				},
				Validation: gatewayv1alpha3.BackendTLSPolicyValidation{
					// Note: WellKnownCACertificates and Hostname fields are intentionally left empty
					// These fields must be manually configured based on your backend service's TLS setup
				},
			},
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
