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

package kgateway

import (
	"strings"
	"testing"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/kgateway"

	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestApplyBackendProtocolProjectsBackendRefToKgatewayBackend(t *testing.T) {
	grpcProtocol := emitterir.BackendProtocolGRPC
	backendRefPort := gatewayv1.PortNumber(8080)

	httpRouteKey := types.NamespacedName{Namespace: "default", Name: "my-route"}
	policy := emitterir.Policy{
		RuleBackendSources: []emitterir.PolicyIndex{{Rule: 0, Backend: 0}},
		Backends: map[types.NamespacedName]emitterir.Backend{
			backendKeyForService("default", "myservice"): {
				Namespace: "default",
				Name:      "myservice-service-upstream",
				Host:      "myservice.default.svc.cluster.local",
				Port:      8080,
				Protocol:  &grpcProtocol,
			},
		},
	}

	httpRouteCtx := emitterir.HTTPRouteContext{
		HTTPRoute: gatewayv1.HTTPRoute{
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{{
					BackendRefs: []gatewayv1.HTTPBackendRef{{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName("myservice"),
								Port: &backendRefPort,
							},
						},
					}},
				}},
			},
		},
	}

	backends := map[types.NamespacedName]*kgateway.Backend{}
	applyBackendProtocol(policy, "ingress-myservice", httpRouteKey, &httpRouteCtx, backends)

	gotRef := httpRouteCtx.Spec.Rules[0].BackendRefs[0].BackendRef
	if gotRef.Group == nil || string(*gotRef.Group) != BackendGVK.Group {
		t.Fatalf("expected backendRef.group %q, got %v", BackendGVK.Group, gotRef.Group)
	}
	if gotRef.Kind == nil || string(*gotRef.Kind) != BackendGVK.Kind {
		t.Fatalf("expected backendRef.kind %q, got %v", BackendGVK.Kind, gotRef.Kind)
	}
	if gotRef.Name != gatewayv1.ObjectName("myservice-service-upstream") {
		t.Fatalf("expected backendRef.name myservice-service-upstream, got %q", gotRef.Name)
	}
	if gotRef.Port != nil {
		t.Fatalf("expected backendRef.port to be nil after rewrite, got %v", *gotRef.Port)
	}
	if gotRef.Kind != nil && string(*gotRef.Kind) == "GRPCRoute" {
		t.Fatalf("expected backend protocol projection to keep HTTPRoute backend refs, not GRPCRoute")
	}

	backendKey := backendKeyForService("default", "myservice")
	kb, ok := backends[backendKey]
	if !ok {
		t.Fatalf("expected generated Backend %s/%s", backendKey.Namespace, backendKey.Name)
	}
	if kb.Spec.Static == nil || kb.Spec.Static.AppProtocol == nil {
		t.Fatalf("expected generated Backend static.appProtocol to be set")
	}
	if *kb.Spec.Static.AppProtocol != kgateway.AppProtocolGrpc {
		t.Fatalf("expected generated Backend appProtocol %q, got %q", kgateway.AppProtocolGrpc, *kb.Spec.Static.AppProtocol)
	}
}

func TestEmitBackendProtocolPatchNotificationsExplainsNoGRPCRouteProjection(t *testing.T) {
	origNotifications := notifications.NotificationAggr.Notifications
	defer func() {
		notifications.NotificationAggr.Notifications = origNotifications
	}()
	notifications.NotificationAggr.Notifications = map[string][]notifications.Notification{}

	grpcProtocol := emitterir.BackendProtocolGRPC
	backendRefPort := gatewayv1.PortNumber(9090)

	policy := emitterir.Policy{
		BackendProtocol:    &grpcProtocol,
		RuleBackendSources: []emitterir.PolicyIndex{{Rule: 0, Backend: 0}},
	}
	httpRouteKey := types.NamespacedName{Namespace: "default", Name: "my-route"}
	httpCtx := emitterir.HTTPRouteContext{
		HTTPRoute: gatewayv1.HTTPRoute{
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{{
					BackendRefs: []gatewayv1.HTTPBackendRef{{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName("grpc-service"),
								Port: &backendRefPort,
							},
						},
					}},
				}},
			},
		},
	}

	emitBackendProtocolPatchNotifications(
		policy,
		"ingress-grpc",
		httpRouteKey,
		httpCtx,
		map[backendProtoPatchKey]struct{}{},
	)

	got := notifications.NotificationAggr.Notifications["ingress-nginx"]
	if len(got) != 1 {
		t.Fatalf("expected 1 ingress-nginx notification, got %d", len(got))
	}
	if !strings.Contains(got[0].Message, "does not emit a GRPCRoute") {
		t.Fatalf("expected message to explain GRPCRoute is not emitted; got:\n%s", got[0].Message)
	}
}
