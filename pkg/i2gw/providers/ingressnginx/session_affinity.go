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
	"strconv"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func sessionAffinityFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	// Iterate over all HTTPRoutes to find backend services and apply generic SessionAffinity
	for _, httpRouteCtx := range ir.HTTPRoutes {
		for ruleIdx := range httpRouteCtx.Spec.Rules {
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
			var cookieName string

			for _, source := range sources {
				if val, ok := source.Ingress.Annotations[AffinityAnnotation]; ok && val == "cookie" {
					affinityType = "Cookie"

					// Check for Max Age (Expires)
					if ttlVal, ok := source.Ingress.Annotations[SessionCookieExpiresAnnotation]; ok {
						if ttl, err := strconv.ParseInt(ttlVal, 10, 64); err == nil {
							cookieTTL = &ttl
						}
					}
					// Check for Cookie Name
					if nameVal, ok := source.Ingress.Annotations[SessionCookieNameAnnotation]; ok {
						cookieName = nameVal
					}
					// Only use the first finding or merge? Nginx usually takes first match or last applied.
					// We'll break on first valid "cookie" affinity found.
					break
				}
			}

			if affinityType == "" {
				continue
			}

			// Apply to all backend refs in this rule?
			// Session Affinity is per Backend Service.
			// We need to update the ServiceIR for the referenced services.

			for _, backendRef := range httpRouteCtx.Spec.Rules[ruleIdx].BackendRefs {
				refName := string(backendRef.Name)
				
				svcKey := types.NamespacedName{
					Namespace: httpRouteCtx.HTTPRoute.Namespace, // assumption: same namespace
					Name:      refName,
				}

				if svc, ok := ir.Services[svcKey]; ok {
					if svc.SessionAffinity == nil {
						svc.SessionAffinity = &emitterir.SessionAffinity{}
					}

					svc.SessionAffinity.Type = affinityType
					svc.SessionAffinity.CookieTTLSec = cookieTTL
					svc.SessionAffinity.CookieName = cookieName

					// Update the map
					ir.Services[svcKey] = svc
				} else {
					// Service doesn't exist yet, create it
					svc = providerir.ProviderSpecificServiceIR{
						SessionAffinity: &emitterir.SessionAffinity{
							Type:         affinityType,
							CookieTTLSec: cookieTTL,
							CookieName:   cookieName,
						},
					}
					ir.Services[svcKey] = svc
				}
			}
		}
	}
	return nil
}
