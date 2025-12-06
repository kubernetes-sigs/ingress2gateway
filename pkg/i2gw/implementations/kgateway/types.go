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

package kgateway

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	// TrafficPolicyGVK is the GroupVersionKind for TrafficPolicy.
	TrafficPolicyGVK = schema.GroupVersionKind{
		Group:   "gateway.kgateway.dev",
		Version: "v1alpha1",
		Kind:    "TrafficPolicy",
	}
	// BackendConfigPolicyGVK is the GroupVersionKind for BackendConfigPolicy.
	BackendConfigPolicyGVK = schema.GroupVersionKind{
		Group:   "gateway.kgateway.dev",
		Version: "v1alpha1",
		Kind:    "BackendConfigPolicy",
	}
	HTTPListenerPolicyGVK = schema.GroupVersionKind{
		Group:   "gateway.kgateway.dev",
		Version: "v1alpha1",
		Kind:    "HTTPListenerPolicy",
	}
	// GatewayExtensionGVK is the GroupVersionKind for GatewayExtension.
	GatewayExtensionGVK = schema.GroupVersionKind{
		Group:   "gateway.kgateway.dev",
		Version: "v1alpha1",
		Kind:    "GatewayExtension",
	}
	// BackendGVK is the GroupVersionKind for Backend.
	BackendGVK = schema.GroupVersionKind{
		Group:   "gateway.kgateway.dev",
		Version: "v1alpha1",
		Kind:    "Backend",
	}
)
