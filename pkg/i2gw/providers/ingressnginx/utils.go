/*
Copyright The Kubernetes Authors.

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
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
)

// getNonCanaryIngress returns the first Ingress source that does not have the canary annotation.
// If all sources are canaries, or no sources exist, it returns the first available source (or nil).
// This is used to prioritize the "main" Ingress for reading common annotations.
func getNonCanaryIngress(sources []providerir.BackendSource) *networkingv1.Ingress {
	for _, source := range sources {
		if _, ok := source.Ingress.Annotations[CanaryAnnotation]; !ok {
			return source.Ingress
		}
	}

	// Fallback: If all sources are somehow canaries (invalid nginx config without a primary),
	// we still return an Ingress to read annotations from to avoid panics.
	if len(sources) > 0 {
		return sources[0].Ingress
	}

	return nil
}
