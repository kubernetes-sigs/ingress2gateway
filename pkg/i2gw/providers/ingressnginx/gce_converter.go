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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func gceFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errList field.ErrorList
	for _, ingress := range ingresses {
		affinity := ingress.Annotations[SessionAffinityAnnotation]
		if affinity == "" {
			continue
		}
		if affinity != "cookie" {
			continue
		}

		cookieName := ingress.Annotations[SessionCookieNameAnnotation]
		

		var cookieTTL *int64
		if expires := ingress.Annotations[SessionCookieExpiresAnnotation]; expires != "" {
			if ttl, err := strconv.ParseInt(expires, 10, 64); err == nil {
				cookieTTL = &ttl
			} else {
				errList = append(errList, field.Invalid(field.NewPath("metadata", "annotations", SessionCookieExpiresAnnotation), expires, "must be a valid integer"))
			}
		}

		// Iterate over rules to find relevant services
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				backend := path.Backend
				if backend.Service != nil {
					key := types.NamespacedName{
						Namespace: ingress.Namespace,
						Name:      backend.Service.Name,
					}
					
					// Ensure Service exists in IR
					if _, ok := ir.Services[key]; !ok {
						ir.Services[key] = providerir.ProviderSpecificServiceIR{}
					}
					svcIR := ir.Services[key]
					if svcIR.Gce == nil {
						svcIR.Gce = &gce.ServiceIR{}
					}
					
					// Populate SessionAffinity
					// Note: GKE usually maps "cookie" to "GENERATED_COOKIE".
					// If CookieName is present, effectively it's still GENERATED_COOKIE mechanism but we overload it?
					// Or strictly speaking, if we want custom name, maybe it maps to something else? 
					// I'll stick to GENERATED_COOKIE and set CookieName.
					
					svcIR.Gce.SessionAffinity = &gce.SessionAffinityConfig{
						AffinityType: "GENERATED_COOKIE",
						CookieName:   cookieName,
						CookieTTLSec: cookieTTL,
					}
					
					ir.Services[key] = svcIR
				}
			}
		}
	}
	return errList
}

