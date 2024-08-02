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

package gce

import (
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
	backendconfigv1beta1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1beta1"
)

type storage struct {
	Ingresses map[types.NamespacedName]*networkingv1.Ingress

	// BackendConfig is a GKE Ingress extension, and it is associated to an GKE
	// Ingress through an annotation on the Service `cloud.google.com/backend-config`.
	Services           map[types.NamespacedName]*apiv1.Service
	BackendConfigs     map[types.NamespacedName]*backendconfigv1.BackendConfig
	BetaBackendConfigs map[types.NamespacedName]*backendconfigv1beta1.BackendConfig
}

func newResourcesStorage() *storage {
	return &storage{
		Ingresses:          make(map[types.NamespacedName]*networkingv1.Ingress),
		Services:           make(map[types.NamespacedName]*apiv1.Service),
		BackendConfigs:     make(map[types.NamespacedName]*backendconfigv1.BackendConfig),
		BetaBackendConfigs: make(map[types.NamespacedName]*backendconfigv1beta1.BackendConfig),
	}
}
