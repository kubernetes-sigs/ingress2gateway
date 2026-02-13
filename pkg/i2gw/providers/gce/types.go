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
	frontendConfigKey                      = "networking.gke.io/v1beta1.FrontendConfig"
)

// SupportedGKEGatewayClasses mapped to true for quick validation check
var SupportedGKEGatewayClasses = map[string]bool{
	"gke-l7-global-external-managed":      true,
	"gke-l7-global-external-managed-mc":   true,
	"gke-l7-regional-external-managed":    true,
	"gke-l7-regional-external-managed-mc": true,
	"gke-l7-rilb":                         true,
	"gke-l7-rilb-mc":                      true,
	"gke-l7-gxlb":                         true,
}

var (
	GCPBackendPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "GCPBackendPolicy",
	}

	GCPGatewayPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "GCPGatewayPolicy",
	}

	HealthCheckPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "HealthCheckPolicy",
	}
)
