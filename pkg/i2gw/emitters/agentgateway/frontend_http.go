/*
Copyright 2026 The Kubernetes Authors.

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

package agentgateway

import (
	"fmt"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyFrontendHTTPPolicy projects frontend HTTP listener settings into
// AgentgatewayPolicy.spec.frontend.http.
//
// Agentgateway validates frontend policies only when they target the Gateway
// resource directly, so these settings are tracked separately from the
// HTTPRoute-scoped policy map used by traffic features.
func applyFrontendHTTPPolicy(
	pol emitterir.Policy,
	ingressName string,
	httpRoute gatewayv1.HTTPRoute,
	defaultNamespace string,
	ap map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayPolicy,
	sources map[types.NamespacedName]string,
) (bool, *field.Error) {
	if pol.FrontendHTTP == nil {
		return false, nil
	}

	http := pol.FrontendHTTP
	if http.HTTP1MaxHeaders == nil &&
		http.HTTP1IdleTimeout == nil &&
		http.HTTP2WindowSize == nil &&
		http.HTTP2ConnectionWindowSize == nil &&
		http.HTTP2FrameSize == nil &&
		http.HTTP2KeepaliveInterval == nil &&
		http.HTTP2KeepaliveTimeout == nil {
		return false, nil
	}

	gatewayKey, ok := gatewayParentKeyForHTTPRoute(httpRoute, defaultNamespace)
	if !ok {
		return false, field.Invalid(
			field.NewPath("emitter", "agentgateway", "AgentgatewayPolicy", "frontend", "http"),
			ingressName,
			"frontend HTTP policies require a parent Gateway target",
		)
	}

	if existing, ok := ap[gatewayKey]; ok {
		if !frontendHTTPPoliciesEqual(existing.Spec.Frontend.HTTP, pol.FrontendHTTP) {
			return false, field.Invalid(
				field.NewPath("emitter", "agentgateway", "AgentgatewayPolicy", "frontend", "http"),
				ingressName,
				fmt.Sprintf(
					"frontend HTTP settings conflict on Gateway %s/%s with source Ingress %q; agentgateway only supports Gateway-scoped frontend HTTP policies",
					gatewayKey.Namespace,
					gatewayKey.Name,
					sources[gatewayKey],
				),
			)
		}
		return true, nil
	}

	ap[gatewayKey] = &agentgatewayv1alpha1.AgentgatewayPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-frontend-http", gatewayKey.Name),
			Namespace: gatewayKey.Namespace,
		},
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
			Frontend: &agentgatewayv1alpha1.Frontend{
				HTTP: &agentgatewayv1alpha1.FrontendHTTP{
					HTTP1MaxHeaders:           http.HTTP1MaxHeaders,
					HTTP1IdleTimeout:          http.HTTP1IdleTimeout,
					HTTP2WindowSize:           http.HTTP2WindowSize,
					HTTP2ConnectionWindowSize: http.HTTP2ConnectionWindowSize,
					HTTP2FrameSize:            http.HTTP2FrameSize,
					HTTP2KeepaliveInterval:    http.HTTP2KeepaliveInterval,
					HTTP2KeepaliveTimeout:     http.HTTP2KeepaliveTimeout,
				},
			},
			TargetRefs: []shared.LocalPolicyTargetReferenceWithSectionName{{
				LocalPolicyTargetReference: shared.LocalPolicyTargetReference{
					Group: gatewayv1.Group("gateway.networking.k8s.io"),
					Kind:  gatewayv1.Kind("Gateway"),
					Name:  gatewayv1.ObjectName(gatewayKey.Name),
				},
			}},
		},
	}
	ap[gatewayKey].SetGroupVersionKind(AgentgatewayPolicyGVK)
	sources[gatewayKey] = ingressName

	return true, nil
}

func frontendHTTPPoliciesEqual(
	existing *agentgatewayv1alpha1.FrontendHTTP,
	desired *emitterir.FrontendHTTPPolicy,
) bool {
	if existing == nil || desired == nil {
		return existing == nil && desired == nil
	}

	return int32PointersEqual(existing.HTTP1MaxHeaders, desired.HTTP1MaxHeaders) &&
		durationPointersEqual(existing.HTTP1IdleTimeout, desired.HTTP1IdleTimeout) &&
		int32PointersEqual(existing.HTTP2WindowSize, desired.HTTP2WindowSize) &&
		int32PointersEqual(existing.HTTP2ConnectionWindowSize, desired.HTTP2ConnectionWindowSize) &&
		int32PointersEqual(existing.HTTP2FrameSize, desired.HTTP2FrameSize) &&
		durationPointersEqual(existing.HTTP2KeepaliveInterval, desired.HTTP2KeepaliveInterval) &&
		durationPointersEqual(existing.HTTP2KeepaliveTimeout, desired.HTTP2KeepaliveTimeout)
}

func int32PointersEqual(existing, desired *int32) bool {
	switch {
	case existing == nil && desired != nil:
		return false
	case existing != nil && desired == nil:
		return false
	case existing != nil && desired != nil && *existing != *desired:
		return false
	default:
		return true
	}
}

func durationPointersEqual(existing, desired *metav1.Duration) bool {
	switch {
	case existing == nil && desired != nil:
		return false
	case existing != nil && desired == nil:
		return false
	case existing != nil && desired != nil && existing.Duration != desired.Duration:
		return false
	default:
		return true
	}
}
