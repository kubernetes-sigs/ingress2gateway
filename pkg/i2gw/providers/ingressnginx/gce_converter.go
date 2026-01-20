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
	"os"
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func gceFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	// Iterate over all HTTPRoutes to find backend services and apply GCE specific policies
	for _, httpRouteCtx := range ir.HTTPRoutes {
		for ruleIdx, _ := range httpRouteCtx.Spec.Rules {
			if ruleIdx >= len(httpRouteCtx.RuleBackendSources) {
				continue
			}
			sources := httpRouteCtx.RuleBackendSources[ruleIdx]
			if len(sources) == 0 {
				continue
			}

			// We need to find the backend service for this rule to attach the policy.
			// Currently, we just look at the BackendRefs.
			// Note: This logic assumes we can map back to the service.
			// Ingress-Nginx usually maps path -> backend service.
			// We check the Ingress sources for the annotation.
			
			var affinityType string
			var cookieTTL *int64
			var securityPolicyName string
			
			for _, source := range sources {
				// Check for Session Affinity
				if val, ok := source.Ingress.Annotations[AffinityAnnotation]; ok && val == "cookie" {
					affinityType = "GENERATED_COOKIE"
					
					// Check for Max Age
					if ttlVal, ok := source.Ingress.Annotations[SessionCookieMaxAgeAnnotation]; ok {
						if ttl, err := strconv.ParseInt(ttlVal, 10, 64); err == nil {
							cookieTTL = &ttl
						}
					}
				}

				// Check for Whitelist Source Range
				if val, ok := source.Ingress.Annotations[WhitelistSourceRangeAnnotation]; ok {
					// We cannot create the Cloud Armor Policy automatically via Gateway API output (it requires a separate resource).
					// We generate a deterministic name and warn the user.
					// Naming convention: generated-whitelist-<service-name>
					// Note: We need the service name. We'll derive it from the BackendRef name later in the loop.
					// But we are in the sources loop. We can hold the value and apply it later.
					
					// Temporarily store the raw value or just a flag. The logic needs to be applied per service.
					// Since we can have multiple sources, we take the last valid one (or first).
					// Let's use the provided value from the annotation.
					securityPolicyName = val
				}
			}

			if affinityType == "" && securityPolicyName == "" {
				continue
			}

			// Apply to all backend refs in this rule?
			// Session Affinity in GCE is per Backend Service.
			// We need to update the GceServiceIR for the referenced services.

			for _, backendRef := range httpRouteCtx.Spec.Rules[ruleIdx].BackendRefs {
				refName := string(backendRef.Name)
				// refGroup := backendRef.Group // usually nil or core
				// refKind := backendRef.Kind // usually Service

				// We need to find the ServiceIR for this backend.
				// In ProviderIR, Services map key is NamespacedName.
				// We assume BackendRef name is the service name in the same namespace (usually).
				
				// Wait, HTTPRouteCtx doesn't easily give us the namespace of the service if it's cross-namespace?
				// But we can check ir.Services.
				
				svcKey := types.NamespacedName{
					Namespace: httpRouteCtx.HTTPRoute.Namespace, // assumption: same namespace
					Name:      refName,
				}

				generatedPolicyName := ""
				if securityPolicyName != "" {
					generatedPolicyName = fmt.Sprintf("generated-whitelist-%s", refName)
					// Verify if we already warned for this service/policy to avoid spam?
					// Simple implementation: just log.
					// Use os.Stderr to ensure it's seen.
					fmt.Fprintf(os.Stderr, "[WARN] Detected whitelist-source-range for service '%s' with values '%s'.\n"+
						"       GKE Gateway requires a pre-existing Cloud Armor Policy.\n"+
						"       Action Required: Create a Cloud Armor Policy named '%s' with these rules.\n",
						refName, securityPolicyName, generatedPolicyName)
				}
				
				if svc, ok := ir.Services[svcKey]; ok {
					if svc.Gce == nil {
						svc.Gce = &gce.ServiceIR{}
					}
					// Session Affinity Update
					if affinityType != "" {
						if svc.Gce.SessionAffinity == nil {
							svc.Gce.SessionAffinity = &gce.SessionAffinityConfig{}
						}
						svc.Gce.SessionAffinity.AffinityType = affinityType
						svc.Gce.SessionAffinity.CookieTTLSec = cookieTTL
					}
					// Security Policy Update
					if generatedPolicyName != "" {
						if svc.Gce.SecurityPolicy == nil {
							svc.Gce.SecurityPolicy = &gce.SecurityPolicyConfig{}
						}
						svc.Gce.SecurityPolicy.Name = generatedPolicyName
					}
					
					// Update the map
					ir.Services[svcKey] = svc
				} else {
					// Service doesn't exist yet, create it
					svc = providerir.ProviderSpecificServiceIR{
						Gce: &gce.ServiceIR{},
					}
					
					if affinityType != "" {
						svc.Gce.SessionAffinity = &gce.SessionAffinityConfig{
							AffinityType: affinityType,
							CookieTTLSec: cookieTTL,
						}
					}
					
					if generatedPolicyName != "" {
						svc.Gce.SecurityPolicy = &gce.SecurityPolicyConfig{
							Name: generatedPolicyName,
						}
					}
					
					ir.Services[svcKey] = svc
				}
			}
		}
	}
	return nil
}
