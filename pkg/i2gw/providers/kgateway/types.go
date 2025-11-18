package kgateway

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	TrafficPolicyGVK = schema.GroupVersionKind{
		Group:   "gateway.kgateway.dev",
		Version: "v1alpha1",
		Kind:    "TrafficPolicy",
	}
)
