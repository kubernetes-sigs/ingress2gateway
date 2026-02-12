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

package emitterir

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// EmitterIR holds specifications of Gateway Objects for supporting Ingress extensions,
// annotations, and proprietary API features not supported as Gateway core
// features. An EmitterIR field can be mapped to core Gateway-API fields,
// or provider-specific Gateway extensions.
type EmitterIR struct {
	Gateways   map[types.NamespacedName]GatewayContext
	HTTPRoutes map[types.NamespacedName]HTTPRouteContext

	GatewayClasses map[types.NamespacedName]GatewayClassContext
	TLSRoutes      map[types.NamespacedName]TLSRouteContext
	TCPRoutes      map[types.NamespacedName]TCPRouteContext
	UDPRoutes      map[types.NamespacedName]UDPRouteContext
	GRPCRoutes     map[types.NamespacedName]GRPCRouteContext

	BackendTLSPolicies map[types.NamespacedName]BackendTLSPolicyContext
	ReferenceGrants    map[types.NamespacedName]ReferenceGrantContext

	GceServices map[types.NamespacedName]gce.ServiceIR
}

type GatewayContext struct {
	gatewayv1.Gateway
	// Emitter IR should be provider/emitter neutral,
	// But we have GCE for backcompatibility.
	Gce *gce.GatewayIR
}

type HTTPRouteContext struct {
	gatewayv1.HTTPRoute
	// TCPTimeoutsByRuleIdx holds provider TCP-level timeouts by HTTPRoute rule index.
	TCPTimeoutsByRuleIdx map[int]*TCPTimeouts

	// PathRewriteByRuleIdx maps HTTPRoute rule indices to path rewrite intent.
	// This is provider-neutral and applied by the common emitter.
	PathRewriteByRuleIdx map[int]*PathRewrite

	// BodySizeByRuleIdx maps HTTPRoute rule indices to body size intent.
	// This is provider-neutral and applied by each custom emitter.
	BodySizeByRuleIdx map[int]*BodySize

	// CorsPolicyByRuleIdx maps HTTPRoute rule indices to CORS policy intent.
	// This map is populated by providers that support CORS (e.g., via annotations) and is
	// applied by the CommonEmitter. This separation allows the CORS logic to be provider-neutral
	// and consistently applied across different providers, subject to feature gating.
	CorsPolicyByRuleIdx map[int]*gatewayv1.HTTPCORSFilter
}

// TCPTimeouts holds TCP-level timeout configuration for a single HTTPRoute rule.
type TCPTimeouts struct {
	Connect *gatewayv1.Duration
	Read    *gatewayv1.Duration
	Write   *gatewayv1.Duration
}

// PathRewrite represents provider-neutral path rewrite intent.
// For now it only supports full-path replacement; more fields may be added later.
type PathRewrite struct {
	ReplaceFullPath string
	// Headers to add on path rewrite.
	Headers                     map[string]string
	RegexCaptureGroupReferences bool
}

// BodySize represents provider-neutral body size intent.
type BodySize struct {
	BufferSize *resource.Quantity
	MaxSize    *resource.Quantity
}

type GatewayClassContext struct {
	gatewayv1.GatewayClass
}

type TLSRouteContext struct {
	gatewayv1alpha2.TLSRoute
}

type TCPRouteContext struct {
	gatewayv1alpha2.TCPRoute
}

type UDPRouteContext struct {
	gatewayv1alpha2.UDPRoute
}

type GRPCRouteContext struct {
	gatewayv1.GRPCRoute
}

type BackendTLSPolicyContext struct {
	gatewayv1.BackendTLSPolicy
}

type ReferenceGrantContext struct {
	gatewayv1beta1.ReferenceGrant
}
