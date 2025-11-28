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

package common

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type uniqueBackendRefsKey struct {
	Name      gatewayv1.ObjectName
	Namespace gatewayv1.Namespace
	Port      gatewayv1.PortNumber
	Group     gatewayv1.Group
	Kind      gatewayv1.Kind
}

// removeBackendRefsDuplicates removes duplicate backendRefs from a list of backendRefs.
func removeBackendRefsDuplicates(backendRefs []gatewayv1.HTTPBackendRef) []gatewayv1.HTTPBackendRef {

	uniqueBackendRefs := map[uniqueBackendRefsKey]*gatewayv1.HTTPBackendRef{}

	for _, backendRef := range backendRefs {
		var k uniqueBackendRefsKey

		group := gatewayv1.Group("")
		kind := gatewayv1.Kind("Service")

		if backendRef.Group != nil && *backendRef.Group != "core" {
			group = *backendRef.Group
		}

		if backendRef.Kind != nil {
			kind = *backendRef.Kind
		}

		k.Name = backendRef.Name
		k.Group = group
		k.Kind = kind

		if backendRef.Port != nil {
			k.Port = *backendRef.Port
		}

		if oldRef, exists := uniqueBackendRefs[k]; exists {
			if oldRef.Weight != nil && backendRef.Weight != nil {
				*oldRef.Weight += *backendRef.Weight
			}
		} else {
			uniqueBackendRefs[k] = backendRef.DeepCopy()
		}
	}
	result := make([]gatewayv1.HTTPBackendRef, 0, len(uniqueBackendRefs))
	for _, backendRef := range uniqueBackendRefs {
		result = append(result, *backendRef)
	}
	return result
}
