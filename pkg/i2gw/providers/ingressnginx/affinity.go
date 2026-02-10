/*
Copyright 2024 The Kubernetes Authors.

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

// affinityFeature parses the ingress-nginx affinity annotations and populates
// the ProviderSpecificServiceIR.Gce.SessionAffinity.
func affinityFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	for _, httpRouteCtx := range ir.HTTPRoutes {
		for ruleIdx, rule := range httpRouteCtx.Spec.Rules {
			if ruleIdx >= len(httpRouteCtx.RuleBackendSources) {
				continue
			}
			sources := httpRouteCtx.RuleBackendSources[ruleIdx]
			if len(sources) == 0 {
				continue
			}

			// Check if the source ingress has the affinity annotation.
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			affinityVal, ok := ingress.Annotations[AffinityAnnotation]
			if !ok || affinityVal != "cookie" {
				// Only "cookie" affinity is supported for GCE.
				continue
			}

			var cookieTTLSecPtr *int64
			maxAgeVal, ok := ingress.Annotations[SessionCookieMaxAgeAnnotation]
			if ok && maxAgeVal != "" {
				parsedTTL, err := strconv.ParseInt(maxAgeVal, 10, 64)
				if err != nil {
					errs = append(errs, field.Invalid(
						field.NewPath("metadata", "annotations", SessionCookieMaxAgeAnnotation),
						maxAgeVal,
						"must be an integer",
					))
				} else if parsedTTL < 0 || parsedTTL > 1209600 {
					errs = append(errs, field.Invalid(
						field.NewPath("metadata", "annotations", SessionCookieMaxAgeAnnotation),
						maxAgeVal,
						"must be between 0 and 1209600 seconds",
					))
				} else {
					cookieTTLSecPtr = &parsedTTL
				}
			}

			for _, backendRef := range rule.BackendRefs {
				if backendRef.Kind != nil && *backendRef.Kind != "Service" {
					continue
				}

				svcName := string(backendRef.Name)
				svcNamespace := ingress.Namespace
				if backendRef.Namespace != nil {
					svcNamespace = string(*backendRef.Namespace)
				}

				svcKey := types.NamespacedName{Namespace: svcNamespace, Name: svcName}

				svcCtx := ir.Services[svcKey]
				if svcCtx.Gce == nil {
					svcCtx.Gce = &gce.ServiceIR{}
				}

				svcCtx.Gce.SessionAffinity = &gce.SessionAffinityConfig{
					AffinityType: "GENERATED_COOKIE",
					CookieTTLSec: cookieTTLSecPtr,
				}
				ir.Services[svcKey] = svcCtx
			}
		}
	}
	return errs
}
