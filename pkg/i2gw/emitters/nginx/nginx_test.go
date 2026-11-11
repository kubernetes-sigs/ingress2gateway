/*
Copyright The Kubernetes Authors.

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

package nginx

import (
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type expectedPolicy struct {
	name      string
	namespace string
	target    string
	maxSize   string
}

func TestEmitClientSettingsPolicies(t *testing.T) {
	t.Parallel()
	routeA := types.NamespacedName{
		Namespace: "default",
		Name:      "route-a",
	}
	routeB := types.NamespacedName{
		Namespace: "default",
		Name:      "route-b",
	}
	otherRoute := types.NamespacedName{
		Namespace: "other",
		Name:      "route-other",
	}

	tests := []struct {
		name              string
		ir                emitterir.EmitterIR
		wantPolicies      []expectedPolicy
		wantNotifications []notifications.MessageType
		wantIRClearedFor  []types.NamespacedName
		wantIRRetainedFor []types.NamespacedName
	}{
		{
			name: "one rule and one body size",
			ir: emitterir.EmitterIR{
				HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
					routeA: routeContext(routeA, 1, map[int]*emitterir.BodySize{
						0: {MaxSize: quantity("60k")},
					}),
				},
			},
			wantPolicies: []expectedPolicy{
				{
					name:      "route-a-client-settings",
					namespace: "default",
					target:    "route-a",
					maxSize:   "60k",
				},
			},
			wantIRClearedFor: []types.NamespacedName{routeA},
		},
		{
			name: "multiple rules, identical sizes",
			ir: emitterir.EmitterIR{
				HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
					routeA: routeContext(routeA, 2, map[int]*emitterir.BodySize{
						0: {MaxSize: quantity("60k")},
						1: {MaxSize: quantity("60k")},
					}),
				},
			},
			wantPolicies: []expectedPolicy{
				{
					name:      "route-a-client-settings",
					namespace: "default",
					target:    "route-a",
					maxSize:   "60k",
				},
			},
			wantIRClearedFor: []types.NamespacedName{routeA},
		},
		{
			name: "multiple rules with conflicting sizes",
			ir: emitterir.EmitterIR{
				HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
					routeA: routeContext(routeA, 2, map[int]*emitterir.BodySize{
						0: {MaxSize: quantity("60k")},
						1: {MaxSize: quantity("100k")},
					}),
				},
			},
			wantNotifications: []notifications.MessageType{
				notifications.ErrorNotification,
			},
			wantIRRetainedFor: []types.NamespacedName{routeA},
		},
		{
			name: "Multiple routes and namespaces",
			ir: emitterir.EmitterIR{
				HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
					routeB: routeContext(routeB, 1, map[int]*emitterir.BodySize{
						0: {MaxSize: quantity("100k")},
					}),
					otherRoute: routeContext(otherRoute, 1, map[int]*emitterir.BodySize{
						0: {MaxSize: quantity("1m")},
					}),
				},
			},
			wantPolicies: []expectedPolicy{
				{
					name:      "route-b-client-settings",
					namespace: "default",
					target:    "route-b",
					maxSize:   "100k",
				},
				{
					name:      "route-other-client-settings",
					namespace: "other",
					target:    "route-other",
					maxSize:   "1m",
				},
			},
			wantIRClearedFor: []types.NamespacedName{
				routeB,
				otherRoute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var gotNotifications []notifications.Notification

			notify := func(level notifications.MessageType, message string, objects ...client.Object) {
				gotNotifications = append(gotNotifications, notifications.Notification{
					Type:           level,
					Message:        message,
					CallingObjects: objects,
				})
			}
			emitter := &Emitter{
				builderMap: NewBuilderMap(),
				notify:     notify,
			}

			gatewayResources := i2gw.GatewayResources{}
			emitter.emitClientSettingsPolicies(tt.ir, &gatewayResources)

			if len(gatewayResources.GatewayExtensions) != len(tt.wantPolicies) {
				t.Fatalf("expected %d policies but got %d",
					len(tt.wantPolicies),
					len(gatewayResources.GatewayExtensions),
				)
			}

			for i, want := range tt.wantPolicies {
				t.Run("policy-"+want.name, func(t *testing.T) {
					t.Parallel()
					policy := gatewayResources.GatewayExtensions[i]

					if policy.GetAPIVersion() != "gateway.nginx.org/v1alpha1" {
						t.Errorf(
							"expected API version %q, got %q",
							"gateway.nginx.org/v1alpha1",
							policy.GetAPIVersion(),
						)
					}

					if policy.GetKind() != "ClientSettingsPolicy" {
						t.Errorf("Expected %q, got %q",
							"ClientSettingsPolicy",
							policy.GetKind(),
						)
					}

					if policy.GetName() != want.name {
						t.Errorf("Expected name %q, but got %q", want.name, policy.GetName())
					}

					if policy.GetNamespace() != want.namespace {
						t.Errorf("Expected namespace %q, but got %q", want.namespace, policy.GetNamespace())
					}

					targetName, found, err := unstructured.NestedString(
						policy.Object,
						"spec", "targetRef", "name",
					)

					if err != nil {
						t.Fatalf("reading targetRef.name: %v", err)
					}

					if !found {
						t.Fatalf("targetRef.name was not emitted")
					}

					if targetName != want.target {
						t.Errorf("expected targetRef.name %q, but got %q", want.target, targetName)
					}

					maxSize, found, err := unstructured.NestedString(
						policy.Object,
						"spec", "body", "maxSize",
					)

					if err != nil {
						t.Fatalf("reading body.mazSize %v", err)
					}

					if !found {
						t.Fatalf("body.maxSize was not emitted")
					}

					if maxSize != want.maxSize {
						t.Errorf(
							"expexted maxSize %q, but got %q",
							want.maxSize,
							maxSize,
						)
					}
				})
			}

			for _, key := range tt.wantIRClearedFor {
				if bodySizes := tt.ir.HTTPRoutes[key].BodySizeByRuleIdx; bodySizes != nil {
					t.Errorf(
						"expected BodySizeByRuleIdx for %s to be cleared, but got %v",
						key,
						bodySizes,
					)
				}
			}

			for _, key := range tt.wantIRRetainedFor {
				if bodySizes := tt.ir.HTTPRoutes[key].BodySizeByRuleIdx; bodySizes == nil {
					t.Errorf(
						"expected BodySizeByRuleIdx for %s to be retained",
						key,
					)
				}
			}

			if len(gotNotifications) != len(tt.wantNotifications) {
				t.Fatalf(
					"expected %d notifications, but got %d",
					len(tt.wantNotifications),
					len(gotNotifications),
				)
			}

			for i, wantType := range tt.wantNotifications {
				if gotNotifications[i].Type != wantType {
					t.Errorf(
						"notification %d: expected type %v, but got %v",
						i,
						wantType,
						gotNotifications[i].Type,
					)
				}
			}
		})
	}
}

func routeContext(key types.NamespacedName, ruleCount int, bodySizes map[int]*emitterir.BodySize) emitterir.HTTPRouteContext {
	return emitterir.HTTPRouteContext{
		HTTPRoute: gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: make([]gatewayv1.HTTPRouteRule, ruleCount),
			},
		},
		BodySizeByRuleIdx: bodySizes,
	}
}

func quantity(value string) *resource.Quantity {
	q := resource.MustParse(value)
	return &q
}
