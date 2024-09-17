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

import (
	"errors"
	"testing"

	gkegatewayv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
	testGatewayName             = "test-gateway"
	testHTTPRouteName           = "test-http-route"
	testSaGCPBackendPolicyName  = testServiceName
	testSslGCPGatewayPolicyName = testGatewayName
)

var (
	testGateway = gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: testGatewayName, Namespace: testNamespace},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
			Listeners: []gatewayv1.Listener{{
				Name:     "test-mydomain-com-http",
				Port:     80,
				Protocol: gatewayv1.HTTPProtocolType,
				Hostname: common.PtrTo(gatewayv1.Hostname(testHost)),
			}},
		},
	}

	testHTTPRoute = gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: testHTTPRouteName, Namespace: testNamespace},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name: gatewayv1.ObjectName(testGatewayName),
				}},
			},
			Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  common.PtrTo(gPathPrefix),
								Value: common.PtrTo("/"),
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(testServiceName),
									Port: common.PtrTo(gatewayv1.PortNumber(80)),
								},
							},
						},
					},
				},
			},
		},
	}

	testSaGCPBackendPolicyCookie = gkegatewayv1.GCPBackendPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.gke.io/v1",
			Kind:       "GCPBackendPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testSaGCPBackendPolicyName,
		},
		Spec: gkegatewayv1.GCPBackendPolicySpec{
			Default: &gkegatewayv1.GCPBackendPolicyConfig{
				SessionAffinity: &gkegatewayv1.SessionAffinityConfig{
					Type:         common.PtrTo("GENERATED_COOKIE"),
					CookieTTLSec: common.PtrTo(testCookieTTLSec),
				},
			},
			TargetRef: v1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  "Service",
				Name:  gatewayv1.ObjectName(testServiceName),
			},
		},
	}

	testSaGCPBackendPolicyClientIP = gkegatewayv1.GCPBackendPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.gke.io/v1",
			Kind:       "GCPBackendPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testSaGCPBackendPolicyName,
		},
		Spec: gkegatewayv1.GCPBackendPolicySpec{
			Default: &gkegatewayv1.GCPBackendPolicyConfig{
				SessionAffinity: &gkegatewayv1.SessionAffinityConfig{
					Type: common.PtrTo("CLIENT_IP"),
				},
			},
			TargetRef: v1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  "Service",
				Name:  gatewayv1.ObjectName(testServiceName),
			},
		},
	}

	testSpGCPBackendPolicy = gkegatewayv1.GCPBackendPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.gke.io/v1",
			Kind:       "GCPBackendPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testSaGCPBackendPolicyName,
		},
		Spec: gkegatewayv1.GCPBackendPolicySpec{
			Default: &gkegatewayv1.GCPBackendPolicyConfig{
				SecurityPolicy: common.PtrTo(testSecurityPolicy),
			},
			TargetRef: v1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  "Service",
				Name:  gatewayv1.ObjectName(testServiceName),
			},
		},
	}

	testSslGCPGatewayPolicy = gkegatewayv1.GCPGatewayPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.gke.io/v1",
			Kind:       "GCPGatewayPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testSslGCPGatewayPolicyName,
		},
		Spec: gkegatewayv1.GCPGatewayPolicySpec{
			Default: &gkegatewayv1.GCPGatewayPolicyConfig{
				SslPolicy: testSslPolicy,
			},
			TargetRef: v1alpha2.NamespacedPolicyTargetReference{
				Group: "gateway.networking.k8s.io",
				Kind:  "Gateway",
				Name:  gatewayv1.ObjectName(testGatewayName),
			},
		},
	}
)

func Test_irToGateway(t *testing.T) {
	testSaGCPBackendPolicyCookieUnstructured, err := i2gw.CastToUnstructured(&testSaGCPBackendPolicyCookie)
	if err != nil {
		t.Errorf("Failed to generate unstructured GCP Backend Policy with Cookie-based session affinity feature: %v", err)
	}
	testSaGCPBackendPolicyClientIPUnstructured, err := i2gw.CastToUnstructured(&testSaGCPBackendPolicyClientIP)
	if err != nil {
		t.Errorf("Failed to generate unstructured GCP Backend Policy with ClientIP-based session affinity feature: %v", err)
	}
	testSpGCPBackendPolicyUnstructured, err := i2gw.CastToUnstructured(&testSpGCPBackendPolicy)
	if err != nil {
		t.Errorf("Failed to generate unstructured GCP Backend Policy with Security Policy feature: %v", err)
	}
	testSslGCPGatewayPolicyUnstructured, err := i2gw.CastToUnstructured(&testSslGCPGatewayPolicy)
	if err != nil {
		t.Errorf("Failed to generate unstructured GCP Gateway Policy with Ssl Policy feature: %v", err)
	}

	testCases := []struct {
		name                     string
		ir                       intermediate.IR
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
	}{
		{
			name: "ingress with a Backend Config specifying CLIENT_IP type session affinity config",
			ir: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: testGatewayName}: {
						Gateway: testGateway,
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: testHTTPRouteName}: {
						HTTPRoute: testHTTPRoute,
					},
				},
				Services: map[types.NamespacedName]intermediate.ProviderSpecificServiceIR{
					{Namespace: testNamespace, Name: testServiceName}: {
						Gce: &intermediate.GceServiceIR{
							SessionAffinity: &intermediate.SessionAffinityConfig{
								AffinityType: saTypeClientIP,
							},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: testGatewayName}: testGateway,
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: testHTTPRouteName}: testHTTPRoute,
				},
				GatewayExtensions: []unstructured.Unstructured{
					*testSaGCPBackendPolicyClientIPUnstructured,
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ingress with a Backend Config specifying GENERATED_COOKIE type session affinity config",
			ir: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: testGatewayName}: {
						Gateway: testGateway,
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: testHTTPRouteName}: {
						HTTPRoute: testHTTPRoute,
					},
				},
				Services: map[types.NamespacedName]intermediate.ProviderSpecificServiceIR{
					{Namespace: testNamespace, Name: testServiceName}: {
						Gce: &intermediate.GceServiceIR{
							SessionAffinity: &intermediate.SessionAffinityConfig{
								AffinityType: saTypeCookie,
								CookieTTLSec: common.PtrTo(testCookieTTLSec),
							},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: testGatewayName}: testGateway,
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: testHTTPRouteName}: testHTTPRoute,
				},
				GatewayExtensions: []unstructured.Unstructured{
					*testSaGCPBackendPolicyCookieUnstructured,
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ingress with a Backend Config specifying Security Policy",
			ir: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: testGatewayName}: {
						Gateway: testGateway,
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: testHTTPRouteName}: {
						HTTPRoute: testHTTPRoute,
					},
				},
				Services: map[types.NamespacedName]intermediate.ProviderSpecificServiceIR{
					{Namespace: testNamespace, Name: testServiceName}: {
						Gce: &intermediate.GceServiceIR{
							SecurityPolicy: &intermediate.SecurityPolicyConfig{
								Name: testSecurityPolicy,
							},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: testGatewayName}: testGateway,
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: testHTTPRouteName}: testHTTPRoute,
				},
				GatewayExtensions: []unstructured.Unstructured{
					*testSpGCPBackendPolicyUnstructured,
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ingress with a Frontend Config specifying Ssl Policy",
			ir: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: testGatewayName}: {
						Gateway: testGateway,
						ProviderSpecificIR: intermediate.ProviderSpecificGatewayIR{
							Gce: &intermediate.GceGatewayIR{
								SslPolicy: &intermediate.SslPolicyConfig{Name: testSslPolicy},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: testHTTPRouteName}: {
						HTTPRoute: testHTTPRoute,
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: testNamespace, Name: testGatewayName}: testGateway,
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: testNamespace, Name: testHTTPRouteName}: testHTTPRoute,
				},
				GatewayExtensions: []unstructured.Unstructured{
					*testSslGCPGatewayPolicyUnstructured,
				},
			},
			expectedErrors: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			provider := NewProvider(&i2gw.ProviderConf{})
			gceProvider := provider.(*Provider)
			gatewayResources, errs := gceProvider.gatewayConverter.irToGateway(tc.ir)

			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
			}

			if len(gatewayResources.HTTPRoutes) != len(tc.expectedGatewayResources.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.HTTPRoutes), len(gatewayResources.HTTPRoutes), gatewayResources.HTTPRoutes)
			} else {
				for i, got := range gatewayResources.HTTPRoutes {
					got.SetGroupVersionKind(common.HTTPRouteGVK)
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.HTTPRoutes[key]
					want.SetGroupVersionKind(common.HTTPRouteGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected HTTPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(gatewayResources.Gateways) != len(tc.expectedGatewayResources.Gateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v",
					len(tc.expectedGatewayResources.Gateways), len(gatewayResources.Gateways), gatewayResources.Gateways)
			} else {
				for i, got := range gatewayResources.Gateways {
					got.SetGroupVersionKind(common.GatewayGVK)
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.Gateways[key]
					want.SetGroupVersionKind(common.GatewayGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(gatewayResources.GatewayExtensions) != len(tc.expectedGatewayResources.GatewayExtensions) {
				t.Errorf("Expected %d GatewayExtensions, got %d: %+v",
					len(tc.expectedGatewayResources.GatewayExtensions), len(gatewayResources.GatewayExtensions), gatewayResources.GatewayExtensions)
			} else {
				for _, got := range gatewayResources.GatewayExtensions {
					for _, want := range tc.expectedGatewayResources.GatewayExtensions {
						if got.GetNamespace() != want.GetNamespace() || got.GetName() != want.GetName() {
							continue
						}
						if !apiequality.Semantic.DeepEqual(got, want) {
							t.Errorf("Expected GatewayExtension %s/%s to be %+v\n Got: %+v\n Diff: %s", got.GetNamespace(), got.GetName(), want, got, cmp.Diff(want, got))
						}
					}
				}
			}
		})
	}
}
