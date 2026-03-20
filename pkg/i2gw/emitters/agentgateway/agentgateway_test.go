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

package agentgateway_emitter

import (
	"testing"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestEmit_Gateway(t *testing.T) {
	e := &Emitter{notify: notifications.NoopNotify}
	nn := types.NamespacedName{Namespace: "default", Name: "test-gateway"}

	gr, errs := e.Emit(emitterir.EmitterIR{
		Gateways: map[types.NamespacedName]emitterir.GatewayContext{
			nn: {
				Gateway: gatewayv1.Gateway{
					Spec: gatewayv1.GatewaySpec{
						Listeners: []gatewayv1.Listener{{
							Name:     "http",
							Port:     80,
							Protocol: gatewayv1.HTTPProtocolType,
							Hostname: common.PtrTo(gatewayv1.Hostname("example.com")),
						}},
					},
				},
			},
		},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if gw, ok := gr.Gateways[nn]; !ok {
		t.Fatalf("missing gateway %s", nn)
	} else if gw.Spec.GatewayClassName != emitterName {
		t.Errorf("unexpected GatewayClassName %q", gw.Spec.GatewayClassName)
	}
}

func TestEmit_BodySize(t *testing.T) {
	nn := types.NamespacedName{Namespace: "default", Name: "test-http-route"}

	testHTTPRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-http-route"},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name: gatewayv1.ObjectName("test-gateway"),
				}},
			},
			Hostnames: []gatewayv1.Hostname{"example.com"},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{{
						Path: &gatewayv1.HTTPPathMatch{
							Type:  common.PtrTo(gatewayv1.PathMatchPathPrefix),
							Value: common.PtrTo("/"),
						},
					}},
					BackendRefs: []gatewayv1.HTTPBackendRef{{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName("test-service"),
								Port: common.PtrTo(gatewayv1.PortNumber(80)),
							},
						},
					}},
				},
			},
		},
	}

	// Limits cannot be expressed on HTTPRoute targets in AgentgatewayPolicy because spec.frontend is Gateway-only
	testCases := []struct {
		name         string
		ir           emitterir.EmitterIR
		wantWarnings int
	}{
		{
			name: "single rule body size emits warning and no policy",
			ir: emitterir.EmitterIR{
				HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
					nn: {
						HTTPRoute: testHTTPRoute,
						BodySizeByRuleIdx: map[int]*emitterir.BodySize{
							0: {BufferSize: common.PtrTo(resource.MustParse("1Mi"))},
						},
					},
				},
			},
			wantWarnings: 1,
		},
		{
			name: "multiple rules each emit a warning",
			ir: emitterir.EmitterIR{
				HTTPRoutes: map[types.NamespacedName]emitterir.HTTPRouteContext{
					nn: {
						HTTPRoute: testHTTPRoute,
						BodySizeByRuleIdx: map[int]*emitterir.BodySize{
							0:                 {BufferSize: common.PtrTo(resource.MustParse("1Mi"))},
							routeRuleAllIndex: {BufferSize: common.PtrTo(resource.MustParse("4Mi"))},
						},
					},
				},
			},
			wantWarnings: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var warnings int
			notify := func(level notifications.MessageType, _ string, _ ...client.Object) {
				if level == notifications.WarningNotification {
					warnings++
				}
			}
			e := &Emitter{notify: notify}
			got, errs := e.Emit(tc.ir)
			if len(errs) != 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if len(got.GatewayExtensions) != 0 {
				t.Errorf("want 0 GatewayExtensions, got %d", len(got.GatewayExtensions))
			}
			if warnings != tc.wantWarnings {
				t.Errorf("want %d warnings, got %d", tc.wantWarnings, warnings)
			}
		})
	}
}
