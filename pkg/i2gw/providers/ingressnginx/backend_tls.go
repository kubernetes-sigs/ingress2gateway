/*
Copyright 2026 The Kubernetes Authors.

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

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// backendTLSFeature processes backend TLS annotations (proxy-ssl-*) and creates
// BackendTLSPolicy resources for services that require TLS connections to the backend.
func backendTLSFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	for _, ingress := range ingresses {
		parseErrs := processBackendTLSAnnotations(&ingress, ir)
		errs = append(errs, parseErrs...)
	}

	return errs
}

// processBackendTLSAnnotations processes the proxy-ssl-* annotations on an ingress
// and creates BackendTLSPolicy resources as needed.
func processBackendTLSAnnotations(ingress *networkingv1.Ingress, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	// Check if any backend TLS annotation is present
	if !hasBackendTLSAnnotations(ingress) {
		return nil
	}

	// Initialize BackendTLSPolicies map if needed
	if ir.BackendTLSPolicies == nil {
		ir.BackendTLSPolicies = make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy)
	}

	// Collect all backend services from the ingress
	services := collectBackendServices(ingress)
	if len(services) == 0 {
		return nil
	}

	// Parse the annotations
	config, parseErrs := parseBackendTLSConfig(ingress)
	if len(parseErrs) > 0 {
		return parseErrs
	}

	// Warn about unsupported annotations
	warnUnsupportedBackendTLSAnnotations(ingress, config)

	// Create BackendTLSPolicy for each service
	for serviceName := range services {
		policy := createBackendTLSPolicy(ingress, serviceName, config)
		policyKey := types.NamespacedName{
			Namespace: ingress.Namespace,
			Name:      policy.Name,
		}

		// Check for conflicting policies from different ingresses
		if existingPolicy, exists := ir.BackendTLSPolicies[policyKey]; exists {
			// Check if the configurations conflict
			if existingPolicy.Spec.Validation.Hostname != policy.Spec.Validation.Hostname {
				notify(notifications.WarningNotification,
					fmt.Sprintf("BackendTLSPolicy %s already exists with different hostname (%s vs %s); using configuration from ingress %s/%s",
						policyKey.Name,
						existingPolicy.Spec.Validation.Hostname,
						policy.Spec.Validation.Hostname,
						ingress.Namespace, ingress.Name),
					ingress)
			}
		}

		ir.BackendTLSPolicies[policyKey] = policy
	}

	return errs
}

// backendTLSConfig holds the parsed values from backend TLS annotations.
type backendTLSConfig struct {
	// Secret containing client certificate (namespace/name format)
	Secret string
	// SecretNamespace is the namespace part of the secret reference
	SecretNamespace string
	// SecretName is the name part of the secret reference
	SecretName string
	// SSL ciphers to use
	Ciphers string
	// SNI hostname to use for backend connections
	Name string
	// SSL protocols to use
	Protocols string
	// Whether to verify backend certificate ("on" or "off")
	Verify string
	// Certificate verification depth
	VerifyDepth string
	// Whether to enable SNI ("on" or "off")
	ServerName string
}

// hasBackendTLSAnnotations checks if the ingress has any backend TLS annotations.
func hasBackendTLSAnnotations(ingress *networkingv1.Ingress) bool {
	if ingress.Annotations == nil {
		return false
	}

	annotations := []string{
		ProxySSLSecretAnnotation,
		ProxySSLCiphersAnnotation,
		ProxySSLNameAnnotation,
		ProxySSLProtocolsAnnotation,
		ProxySSLVerifyAnnotation,
		ProxySSLVerifyDepthAnnotation,
		ProxySSLServerNameAnnotation,
	}

	for _, ann := range annotations {
		if _, ok := ingress.Annotations[ann]; ok {
			return true
		}
	}
	return false
}

// parseBackendTLSConfig extracts backend TLS configuration from ingress annotations.
func parseBackendTLSConfig(ingress *networkingv1.Ingress) (backendTLSConfig, field.ErrorList) {
	var errs field.ErrorList

	config := backendTLSConfig{
		Secret:      ingress.Annotations[ProxySSLSecretAnnotation],
		Ciphers:     ingress.Annotations[ProxySSLCiphersAnnotation],
		Name:        ingress.Annotations[ProxySSLNameAnnotation],
		Protocols:   ingress.Annotations[ProxySSLProtocolsAnnotation],
		Verify:      ingress.Annotations[ProxySSLVerifyAnnotation],
		VerifyDepth: ingress.Annotations[ProxySSLVerifyDepthAnnotation],
		ServerName:  ingress.Annotations[ProxySSLServerNameAnnotation],
	}

	// Parse and validate secret reference if provided
	if config.Secret != "" {
		secretNamespace, secretName, err := parseSecretReference(config.Secret, ingress.Namespace)
		if err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations", ProxySSLSecretAnnotation),
				config.Secret,
				err.Error(),
			))
		} else {
			config.SecretNamespace = secretNamespace
			config.SecretName = secretName
		}
	}

	return config, errs
}

// parseSecretReference parses a secret reference in the format "namespace/name" or "name".
// Returns the namespace and name, using defaultNamespace if not specified.
func parseSecretReference(ref, defaultNamespace string) (namespace, name string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("empty secret reference")
	}

	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		namespace = strings.TrimSpace(parts[0])
		name = strings.TrimSpace(parts[1])
		if namespace == "" {
			return "", "", fmt.Errorf("empty namespace in secret reference %q", ref)
		}
		if name == "" {
			return "", "", fmt.Errorf("empty name in secret reference %q", ref)
		}
	} else {
		namespace = defaultNamespace
		name = strings.TrimSpace(parts[0])
		if name == "" {
			return "", "", fmt.Errorf("empty secret name")
		}
	}

	return namespace, name, nil
}

// collectBackendServices extracts all unique backend service names from an ingress.
func collectBackendServices(ingress *networkingv1.Ingress) map[string]struct{} {
	services := make(map[string]struct{})

	// Check default backend
	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		services[ingress.Spec.DefaultBackend.Service.Name] = struct{}{}
	}

	// Check rule backends
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				services[path.Backend.Service.Name] = struct{}{}
			}
		}
	}

	return services
}

// warnUnsupportedBackendTLSAnnotations emits warnings for annotations that cannot
// be fully converted to Gateway API BackendTLSPolicy.
func warnUnsupportedBackendTLSAnnotations(ingress *networkingv1.Ingress, config backendTLSConfig) {
	var warnings []string

	if config.Ciphers != "" {
		warnings = append(warnings, fmt.Sprintf("%s: SSL cipher configuration is not supported by Gateway API BackendTLSPolicy", ProxySSLCiphersAnnotation))
	}

	if config.Protocols != "" {
		warnings = append(warnings, fmt.Sprintf("%s: SSL protocol configuration is not supported by Gateway API BackendTLSPolicy", ProxySSLProtocolsAnnotation))
	}

	if config.VerifyDepth != "" {
		warnings = append(warnings, fmt.Sprintf("%s: certificate verification depth is not supported by Gateway API BackendTLSPolicy", ProxySSLVerifyDepthAnnotation))
	}

	if config.ServerName != "" {
		warnings = append(warnings, fmt.Sprintf("%s: server name (SNI) on/off toggle is not supported; SNI is always used when hostname is set", ProxySSLServerNameAnnotation))
	}

	if config.Secret != "" {
		warnings = append(warnings, fmt.Sprintf("%s: client certificate (mTLS to backend) requires manual configuration of BackendTLSPolicy.Spec.Options or implementation-specific extensions", ProxySSLSecretAnnotation))
	}

	if config.Verify == "on" || config.Verify == "true" {
		warnings = append(warnings, fmt.Sprintf("%s: backend certificate verification enabled; you must manually configure BackendTLSPolicy.Spec.Validation with appropriate CA certificates", ProxySSLVerifyAnnotation))
	}

	for _, warning := range warnings {
		notify(notifications.WarningNotification, warning, ingress)
	}
}

// createBackendTLSPolicy creates a BackendTLSPolicy for a specific service.
// Uses common.CreateBackendTLSPolicy as base and adds hostname configuration.
func createBackendTLSPolicy(ingress *networkingv1.Ingress, serviceName string, config backendTLSConfig) gatewayv1.BackendTLSPolicy {
	policyName := fmt.Sprintf("%s-%s-backend-tls", ingress.Name, serviceName)

	// Use the common helper to create the base policy
	policy := common.CreateBackendTLSPolicy(ingress.Namespace, policyName, serviceName)

	// Set hostname for SNI validation
	// If proxy-ssl-name is provided, use it; otherwise use the service name as a sensible default
	if config.Name != "" {
		policy.Spec.Validation.Hostname = gatewayv1.PreciseHostname(config.Name)
	} else {
		// Use service name as default hostname for validation
		// This ensures the BackendTLSPolicy has a valid Hostname field
		policy.Spec.Validation.Hostname = gatewayv1.PreciseHostname(serviceName)
		notify(notifications.InfoNotification,
			fmt.Sprintf("BackendTLSPolicy for service %s: using service name as default hostname; set %s to specify custom SNI hostname",
				serviceName, ProxySSLNameAnnotation),
			ingress)
	}

	// If verify is disabled, warn about Gateway API limitations
	if config.Verify == "off" || config.Verify == "false" {
		notify(notifications.WarningNotification,
			fmt.Sprintf("%s=off: Gateway API BackendTLSPolicy does not support disabling certificate verification; backend TLS will require valid certificates", ProxySSLVerifyAnnotation),
			ingress)
	}

	// Add info notification about secret reference if provided
	if config.SecretName != "" {
		notify(notifications.InfoNotification,
			fmt.Sprintf("BackendTLSPolicy created for service %s; client certificate from secret (namespace=%s, name=%s) requires manual configuration",
				serviceName, config.SecretNamespace, config.SecretName),
			ingress)
	}

	return policy
}
