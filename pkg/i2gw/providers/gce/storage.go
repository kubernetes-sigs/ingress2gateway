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
	frontendconfigv1beta1 "k8s.io/ingress-gce/pkg/apis/frontendconfig/v1beta1"
)

type storage struct {
	Ingresses map[types.NamespacedName]*networkingv1.Ingress
	Services  map[types.NamespacedName]*apiv1.Service

	// BackendConfig is a GKE Ingress extension, and it is associated to an GKE
	// Ingress through specifying `cloud.google.com/backend-config` or
	// `beta.cloud.google.com/backend-config` annotation on its Services.
	// BackendConfig map is keyed by the namespaced name of the BackendConfig.
	BackendConfigs map[types.NamespacedName]*backendconfigv1.BackendConfig
	// FrontendConfigs is a GKE Ingress extension, and it is associated to an
	// GKE Ingress through specifying `networking.gke.io/v1beta1.FrontendConfig`
	// on an Ingress.
	// FrontendConfig map is keyed by the namespaced name of the FrontendConfig.
	FrontendConfigs map[types.NamespacedName]*frontendconfigv1beta1.FrontendConfig
}

func newResourcesStorage() *storage {
	return &storage{
		Ingresses:       make(map[types.NamespacedName]*networkingv1.Ingress),
		Services:        make(map[types.NamespacedName]*apiv1.Service),
		BackendConfigs:  make(map[types.NamespacedName]*backendconfigv1.BackendConfig),
		FrontendConfigs: make(map[types.NamespacedName]*frontendconfigv1beta1.FrontendConfig),
	}
}
