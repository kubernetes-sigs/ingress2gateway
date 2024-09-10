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

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	gceIngressClass      = "gce"
	gceL7ILBIngressClass = "gce-internal"

	gceL7GlobalExternalManagedGatewayClass = "gke-l7-global-external-managed"
	gceL7RegionalInternalGatewayClass      = "gke-l7-rilb"
	backendConfigKey                       = "cloud.google.com/backend-config"
	betaBackendConfigKey                   = "beta.cloud.google.com/backend-config"
)

var GCPBackendPolicyGVK = schema.GroupVersionKind{
	Group:   "networking.gke.io",
	Version: "v1",
	Kind:    "GCPBackendPolicy",
}
