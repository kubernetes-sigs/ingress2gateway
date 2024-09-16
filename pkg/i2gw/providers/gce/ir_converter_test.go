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
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Test_convertToIR(t *testing.T) {
	testNamespace := "default"
	testHost := "test.mydomain.com"
	testServiceName := "test-service"
	testBackendConfigName := "test-backendconfig"
	iPrefix := networkingv1.PathTypePrefix
	implSpecificPathType := networkingv1.PathTypeImplementationSpecific

	gPathPrefix := gatewayv1.PathMatchPathPrefix
	gExact := gatewayv1.PathMatchExact

	extIngClassIngressName := "gce-ingress-class"
	intIngClassIngressName := "gce-internal-ingress-class"
	noIngClassIngressName := "no-ingress-class"

	saTypeClientIP := "CLIENT_IP"
	testCookieTTLSec := int64(10)
	saTypeCookie := "GENERATED_COOKIE"
	testSecurityPolicy := "test-security-policy"

	testExtIngress := getTestIngress(testNamespace, extIngClassIngressName, testServiceName, true)
	testIntIngress := getTestIngress(testNamespace, intIngClassIngressName, testServiceName, false)

	testCases := []struct {
		name           string
		ingresses      map[types.NamespacedName]*networkingv1.Ingress
		services       map[types.NamespacedName]*apiv1.Service
		backendConfigs map[types.NamespacedName]*backendconfigv1.BackendConfig
		expectedIR     intermediate.IR
		expectedErrors field.ErrorList
	}{
		{
			name: "gce ingress class",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: testExtIngress,
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							}},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "gce-internal ingress class",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: intIngClassIngressName}: testIntIngress,
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceL7ILBIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceL7ILBIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7RegionalInternalGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: "gce-internal-ingress-class-test-mydomain-com"}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: "gce-internal-ingress-class-test-mydomain-com", Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceL7ILBIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "empty ingress class, default to gce ingress class",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: noIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      noIngClassIngressName,
						Namespace: testNamespace,
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/",
										PathType: &iPrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testServiceName,
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", noIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", noIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "gce implementation-specific with /*, map to / Prefix",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        extIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/*",
										PathType: &implSpecificPathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testServiceName,
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "gce implementation-specific with /foo/*, converted to /foo",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        extIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/foo/*",
										PathType: &implSpecificPathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testServiceName,
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/foo"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: nil,
		},
		{
			name: "gce implementation-specific without wildcard path, map to Prefix",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:        extIngClassIngressName,
						Namespace:   testNamespace,
						Annotations: map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{{
							Host: testHost,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{{
										Path:     "/foo",
										PathType: &implSpecificPathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: testServiceName,
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gExact,
													Value: ptrTo("/foo"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ingress with a Backend Config specifying CLIENT_IP type session affinity config",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: testExtIngress,
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
						Annotations: map[string]string{
							backendConfigKey: `{"default":"test-backendconfig"}`,
						},
					},
				},
			},
			backendConfigs: map[types.NamespacedName]*backendconfigv1.BackendConfig{
				{Namespace: testNamespace, Name: testBackendConfigName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testBackendConfigName,
					},
					Spec: backendconfigv1.BackendConfigSpec{
						SessionAffinity: &backendconfigv1.SessionAffinityConfig{
							AffinityType: saTypeClientIP,
						},
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
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
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ingress with a Backend Config specifying GENERATED_COOKIE type session affinity config",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: testExtIngress,
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
						Annotations: map[string]string{
							backendConfigKey: `{"default":"test-backendconfig"}`,
						},
					},
				},
			},
			backendConfigs: map[types.NamespacedName]*backendconfigv1.BackendConfig{
				{Namespace: testNamespace, Name: testBackendConfigName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testBackendConfigName,
					},
					Spec: backendconfigv1.BackendConfigSpec{
						SessionAffinity: &backendconfigv1.SessionAffinityConfig{
							AffinityType:         saTypeCookie,
							AffinityCookieTtlSec: &testCookieTTLSec,
						},
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Services: map[types.NamespacedName]intermediate.ProviderSpecificServiceIR{
					{Namespace: testNamespace, Name: testServiceName}: {
						Gce: &intermediate.GceServiceIR{
							SessionAffinity: &intermediate.SessionAffinityConfig{
								AffinityType: saTypeCookie,
								CookieTTLSec: &testCookieTTLSec,
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "ingress with a Backend Config specifying Security Policy",
			ingresses: map[types.NamespacedName]*networkingv1.Ingress{
				{Namespace: testNamespace, Name: extIngClassIngressName}: testExtIngress,
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testServiceName,
						Annotations: map[string]string{
							backendConfigKey: `{"default":"test-backendconfig"}`,
						},
					},
				},
			},
			backendConfigs: map[types.NamespacedName]*backendconfigv1.BackendConfig{
				{Namespace: testNamespace, Name: testBackendConfigName}: {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testBackendConfigName,
					},
					Spec: backendconfigv1.BackendConfigSpec{
						SecurityPolicy: &backendconfigv1.SecurityPolicyConfig{
							Name: testSecurityPolicy,
						},
					},
				},
			},
			expectedIR: intermediate.IR{
				Gateways: map[types.NamespacedName]intermediate.GatewayContext{
					{Namespace: testNamespace, Name: gceIngressClass}: {
						Gateway: gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{Name: gceIngressClass, Namespace: testNamespace},
							Spec: gatewayv1.GatewaySpec{
								GatewayClassName: gceL7GlobalExternalManagedGatewayClass,
								Listeners: []gatewayv1.Listener{{
									Name:     "test-mydomain-com-http",
									Port:     80,
									Protocol: gatewayv1.HTTPProtocolType,
									Hostname: ptrTo(gatewayv1.Hostname(testHost)),
								}},
							},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]intermediate.HTTPRouteContext{
					{Namespace: testNamespace, Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName)}: {
						HTTPRoute: gatewayv1.HTTPRoute{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-test-mydomain-com", extIngClassIngressName), Namespace: testNamespace},
							Spec: gatewayv1.HTTPRouteSpec{
								CommonRouteSpec: gatewayv1.CommonRouteSpec{
									ParentRefs: []gatewayv1.ParentReference{{
										Name: gceIngressClass,
									}},
								},
								Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(testHost)},
								Rules: []gatewayv1.HTTPRouteRule{
									{
										Matches: []gatewayv1.HTTPRouteMatch{
											{
												Path: &gatewayv1.HTTPPathMatch{
													Type:  &gPathPrefix,
													Value: ptrTo("/"),
												},
											},
										},
										BackendRefs: []gatewayv1.HTTPBackendRef{
											{
												BackendRef: gatewayv1.BackendRef{
													BackendObjectReference: gatewayv1.BackendObjectReference{
														Name: gatewayv1.ObjectName(testServiceName),
														Port: ptrTo(gatewayv1.PortNumber(80)),
													},
												},
											},
										},
									},
								},
							},
						},
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
			expectedErrors: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			provider := NewProvider(&i2gw.ProviderConf{})
			gceProvider := provider.(*Provider)
			gceProvider.storage = newResourcesStorage()
			gceProvider.storage.Ingresses = tc.ingresses
			gceProvider.storage.Services = tc.services
			gceProvider.storage.BackendConfigs = tc.backendConfigs

			// TODO(#113) we pass an empty i2gw.InputResources temporarily until we change ToIR function on the interface
			ir, errs := gceProvider.irConverter.convertToIR(gceProvider.storage)

			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
			}

			if len(ir.HTTPRoutes) != len(tc.expectedIR.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedIR.HTTPRoutes), len(ir.HTTPRoutes), ir.HTTPRoutes)
			} else {
				for i, gotHTTPRouteContext := range ir.HTTPRoutes {
					key := types.NamespacedName{Namespace: gotHTTPRouteContext.HTTPRoute.Namespace, Name: gotHTTPRouteContext.HTTPRoute.Name}
					wantHTTPRouteContext := tc.expectedIR.HTTPRoutes[key]
					wantHTTPRouteContext.HTTPRoute.SetGroupVersionKind(common.HTTPRouteGVK)
					if !apiequality.Semantic.DeepEqual(gotHTTPRouteContext.HTTPRoute, wantHTTPRouteContext.HTTPRoute) {
						t.Errorf("Expected HTTPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, wantHTTPRouteContext.HTTPRoute, gotHTTPRouteContext.HTTPRoute, cmp.Diff(wantHTTPRouteContext.HTTPRoute, gotHTTPRouteContext.HTTPRoute))
					}
				}
			}

			if len(ir.Gateways) != len(tc.expectedIR.Gateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v",
					len(tc.expectedIR.Gateways), len(ir.Gateways), ir.Gateways)
			} else {
				for i, gotGatewayContext := range ir.Gateways {
					key := types.NamespacedName{Namespace: gotGatewayContext.Gateway.Namespace, Name: gotGatewayContext.Gateway.Name}
					wantGatewayContext := tc.expectedIR.Gateways[key]
					wantGatewayContext.Gateway.SetGroupVersionKind(common.GatewayGVK)
					if !apiequality.Semantic.DeepEqual(gotGatewayContext.Gateway, wantGatewayContext.Gateway) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, wantGatewayContext.Gateway, gotGatewayContext.Gateway, cmp.Diff(wantGatewayContext.Gateway, gotGatewayContext.Gateway))
					}
				}
			}

			if len(ir.Services) != len(tc.expectedIR.Services) {
				t.Errorf("Expected %d ServiceIR, got %d: %+v",
					len(tc.expectedIR.Services), len(ir.Services), ir.Services)
			} else {
				for svcKey, gotServiceIR := range ir.Services {
					key := types.NamespacedName{Namespace: svcKey.Namespace, Name: svcKey.Name}
					wantServiceIR := tc.expectedIR.Services[key]
					if !apiequality.Semantic.DeepEqual(gotServiceIR, wantServiceIR) {
						t.Errorf("Expected ServiceIR %s to be %+v\n Got: %+v\n Diff: %s", svcKey, wantServiceIR, gotServiceIR, cmp.Diff(wantServiceIR, gotServiceIR))
					}
				}
			}
		})
	}
}

func ptrTo[T any](a T) *T {
	return &a
}

func getTestIngress(ingressNamespace, ingressName, serviceName string, isExternalIngress bool) *networkingv1.Ingress {
	iPrefix := networkingv1.PathTypePrefix

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: ingressNamespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: "test.mydomain.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &iPrefix,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: serviceName,
									Port: networkingv1.ServiceBackendPort{
										Number: 80,
									},
								},
							},
						}},
					},
				},
			}},
		},
	}
	if isExternalIngress {
		ing.Annotations = map[string]string{networkingv1beta1.AnnotationIngressClass: gceIngressClass}
	} else {
		ing.Annotations = map[string]string{networkingv1beta1.AnnotationIngressClass: gceL7ILBIngressClass}
	}

	return &ing
}

func TestGetBackendConfigMapping(t *testing.T) {
	t.Parallel()
	testNamespace := "test-namespace"

	testServiceName := "test-service"
	testBeConfigName1 := "backendconfig-1"
	testBeConfigName2 := "backendconfig-2"
	testBeConfigName3 := "backendconfig-3"
	backendConfigs := map[types.NamespacedName]*backendconfigv1.BackendConfig{
		{Namespace: testNamespace, Name: testBeConfigName1}: {},
		{Namespace: testNamespace, Name: testBeConfigName2}: {},
		{Namespace: testNamespace, Name: testBeConfigName3}: {},
	}

	testCases := []struct {
		desc                   string
		serviceAnnotations     map[string]string
		expectedBeConfigToSvcs map[types.NamespacedName]serviceNames
	}{
		{
			desc:                   "No BackendConfig Annotation on Service",
			serviceAnnotations:     map[string]string{},
			expectedBeConfigToSvcs: map[types.NamespacedName]serviceNames{},
		},
		{
			desc: "Specify BackendConfig with cloud.google.com/backend-config annotation, using the same BackendConfig for all ports",
			serviceAnnotations: map[string]string{
				backendConfigKey: `{"default":"backendconfig-1"}`,
			},
			expectedBeConfigToSvcs: map[types.NamespacedName]serviceNames{
				{Namespace: testNamespace, Name: testBeConfigName1}: {
					{Namespace: testNamespace, Name: testServiceName},
				},
			},
		},
		{
			desc: "Specify BackendConfig with beta.cloud.google.com/backend-config annotation, using the same BackendConfig for all ports",
			serviceAnnotations: map[string]string{
				betaBackendConfigKey: `{"default":"backendconfig-1"}`,
			},
			expectedBeConfigToSvcs: map[types.NamespacedName]serviceNames{
				{Namespace: testNamespace, Name: testBeConfigName1}: {
					{Namespace: testNamespace, Name: testServiceName},
				},
			},
		},
		{
			desc: "Specify BackendConfig with cloud.google.com/backend-config annotation, using different BackendConfigs for each port, service will be associated with the BackendConfig for the alphabetically smallest port",
			serviceAnnotations: map[string]string{
				backendConfigKey: `{"ports": {"port1": "backendconfig-2", "port2": "backendconfig-3"}}`,
			},
			expectedBeConfigToSvcs: map[types.NamespacedName]serviceNames{
				// backendconfig-2 has precedence since port1 is alphabetically smaller than port2
				{Namespace: testNamespace, Name: testBeConfigName2}: {
					{Namespace: testNamespace, Name: testServiceName},
				},
			},
		},
		{
			desc: "Specify BackendConfig with beta.cloud.google.com/backend-config annotation, using different BackendConfigs for each port, service will be associated with the BackendConfig for the alphabetically smallest port",
			serviceAnnotations: map[string]string{
				betaBackendConfigKey: `{"ports": {"port1": "backendconfig-2", "port2": "backendconfig-3"}}`,
			},
			expectedBeConfigToSvcs: map[types.NamespacedName]serviceNames{
				// backendconfig-2 has precedence since port1 is alphabetically smaller than port2
				{Namespace: testNamespace, Name: testBeConfigName2}: {
					{Namespace: testNamespace, Name: testServiceName},
				},
			},
		},
		{
			desc: "Specify BackendConfig with both annotations, using the same BackendConfig for all ports, cloud.google.com/backend-config should have precedence over the beta one",
			serviceAnnotations: map[string]string{
				backendConfigKey:     `{"default":"backendconfig-1"}`,
				betaBackendConfigKey: `{"ports": {"port1": "backendconfig-2", "port2": "backendconfig-3"}}`,
			},
			expectedBeConfigToSvcs: map[types.NamespacedName]serviceNames{
				// BackendConfigs in betaBackendConfigKey should be ignored.
				{Namespace: testNamespace, Name: testBeConfigName1}: {
					{Namespace: testNamespace, Name: testServiceName},
				},
			},
		},
		{
			desc: "Specify BackendConfig with both annotations, cloud.google.com/backend-config should have precedence over the beta one, using different BackendConfigs for each port, service will be associated with the BackendConfig for the alphabetically smallest port",
			serviceAnnotations: map[string]string{
				backendConfigKey:     `{"ports": {"port1": "backendconfig-1", "port2": "backendconfig-2"}}`,
				betaBackendConfigKey: `{"default":"backendconfig-3"}`,
			},
			expectedBeConfigToSvcs: map[types.NamespacedName]serviceNames{
				// BackendConfigs in betaBackendConfigKey should be ignored,
				// and backendconfig-1 has precedence since port1 is alphabetically
				// smaller than port2.
				{Namespace: testNamespace, Name: testBeConfigName1}: {
					{Namespace: testNamespace, Name: testServiceName},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			provider := NewProvider(&i2gw.ProviderConf{})
			gceProvider := provider.(*Provider)
			gceProvider.storage = newResourcesStorage()
			testService := apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServiceName,
					Namespace: testNamespace,
				},
			}
			testService.Annotations = tc.serviceAnnotations
			gceProvider.storage.Services = map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: &testService,
			}
			gceProvider.storage.BackendConfigs = backendConfigs

			beConfigToSvcs := getBackendConfigMapping(context.TODO(), gceProvider.storage)
			if !reflect.DeepEqual(beConfigToSvcs, tc.expectedBeConfigToSvcs) {
				t.Errorf("Got BackendConfig mapping %v, expected %v", beConfigToSvcs, tc.expectedBeConfigToSvcs)
			}
		})
	}
}

func TestGetBackendConfigName(t *testing.T) {
	t.Parallel()

	testNamespace := "test-namespace"
	testServiceName := "test-service"
	testBeConfigName := "backendconfig-1"

	testCases := []struct {
		desc           string
		service        *apiv1.Service
		beConfigKey    string
		expectedName   string
		expectedExists bool
	}{
		{
			desc: "Service without BackendConfig annotation, indexing with cloud.google.com/backend-config",
			service: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServiceName,
					Namespace: testNamespace,
				},
			},
			beConfigKey:    backendConfigKey,
			expectedName:   "",
			expectedExists: false,
		},
		{
			desc: "Service without BackendConfig annotation, indexing with beta.cloud.google.com/backend-config",
			service: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServiceName,
					Namespace: testNamespace,
				},
			},
			beConfigKey:    betaBackendConfigKey,
			expectedName:   "",
			expectedExists: false,
		},
		{
			desc: "Service using cloud.google.com/backend-config, using default Config over all ports",
			service: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServiceName,
					Namespace: testNamespace,
					Annotations: map[string]string{
						backendConfigKey: `{"default":"backendconfig-1"}`,
					},
				},
			},
			beConfigKey:    backendConfigKey,
			expectedName:   testBeConfigName,
			expectedExists: true,
		},
		{
			desc: "Service using beta.cloud.google.com/backend-config annotation, using default Config over all ports",
			service: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServiceName,
					Namespace: testNamespace,
					Annotations: map[string]string{
						betaBackendConfigKey: `{"default":"backendconfig-1"}`,
					},
				},
			},
			beConfigKey:    betaBackendConfigKey,
			expectedName:   testBeConfigName,
			expectedExists: true,
		},
		{
			desc: "Service using cloud.google.com/backend-config, using Port Config, pick the BackendConfig with the alphabetically smallest port",
			service: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServiceName,
					Namespace: testNamespace,
					Annotations: map[string]string{
						backendConfigKey: `{"ports": {"port1": "backendconfig-1", "port2": "backendconfig-2"}}`,
					},
				},
			},
			beConfigKey:    backendConfigKey,
			expectedName:   "backendconfig-1",
			expectedExists: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.TODO()
			ctx = context.WithValue(ctx, serviceKey, tc.service)
			gotName, gotExists := getBackendConfigName(ctx, tc.service, tc.beConfigKey)
			if gotExists != tc.expectedExists {
				t.Errorf("getBackendConfigName() got exist = %v, expected %v", gotExists, tc.expectedExists)
			}
			if gotName != tc.expectedName {
				t.Errorf("getBackendConfigName() got exist = %v, expected %v", gotName, tc.expectedName)
			}
		})
	}
}
