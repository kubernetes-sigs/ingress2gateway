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
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"

	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const sourceIngressAnnotation = "ingress2gateway.kubernetes.io/source-ingress"

// applyServiceUpstream projects provider service-upstream backend metadata into
// AgentgatewayBackend CRs and rewrites HTTPRoute backendRefs to target them.
func applyServiceUpstream(
	pol emitterir.Policy,
	ingressName string,
	httpRouteKey types.NamespacedName,
	httpRouteCtx *emitterir.HTTPRouteContext,
	backends map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayBackend,
) {
	if len(pol.Backends) == 0 || len(pol.RuleBackendSources) == 0 {
		return
	}

	for _, idx := range pol.RuleBackendSources {
		if idx.Rule >= len(httpRouteCtx.Spec.Rules) {
			continue
		}
		rule := &httpRouteCtx.Spec.Rules[idx.Rule]
		if idx.Backend >= len(rule.BackendRefs) {
			continue
		}

		br := &rule.BackendRefs[idx.Backend]

		// Only core Services are eligible for service-upstream rewriting.
		if br.BackendRef.Group != nil && *br.BackendRef.Group != "" {
			continue
		}
		if br.BackendRef.Kind != nil && *br.BackendRef.Kind != "Service" {
			continue
		}
		if br.BackendRef.Name == "" {
			continue
		}

		svcName := string(br.BackendRef.Name)
		backendKey := serviceUpstreamBackendKey(httpRouteKey.Namespace, svcName)

		be, ok := pol.Backends[backendKey]
		if !ok {
			continue
		}

		agb := ensureServiceUpstreamBackend(
			ingressName,
			backendKey,
			be.Host,
			be.Port,
			backends,
		)

		// Rewrite HTTPRoute backendRef to point at the AgentgatewayBackend.
		group := gatewayv1.Group(AgentgatewayBackendGVK.Group)
		kind := gatewayv1.Kind(AgentgatewayBackendGVK.Kind)

		br.BackendRef.Group = &group
		br.BackendRef.Kind = &kind
		br.BackendRef.Name = gatewayv1.ObjectName(agb.Name)
		// Port is defined by AgentgatewayBackend.spec.static.
		br.BackendRef.Port = nil
	}
}

func serviceUpstreamBackendKey(ns, svcName string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: ns,
		Name:      svcName + "-service-upstream",
	}
}

func ensureServiceUpstreamBackend(
	ingressName string,
	backendKey types.NamespacedName,
	host string,
	port int32,
	backends map[types.NamespacedName]*agentgatewayv1alpha1.AgentgatewayBackend,
) *agentgatewayv1alpha1.AgentgatewayBackend {
	if be, ok := backends[backendKey]; ok {
		return be
	}

	be := &agentgatewayv1alpha1.AgentgatewayBackend{
		TypeMeta: metav1.TypeMeta{
			Kind:       AgentgatewayBackendGVK.Kind,
			APIVersion: AgentgatewayBackendGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backendKey.Name,
			Namespace: backendKey.Namespace,
			Labels: map[string]string{
				sourceIngressAnnotation: ingressName,
			},
		},
		Spec: agentgatewayv1alpha1.AgentgatewayBackendSpec{
			Static: &agentgatewayv1alpha1.StaticBackend{
				Host: agentgatewayv1alpha1.ShortString(host),
				Port: port,
			},
		},
	}

	backends[backendKey] = be
	return be
}
