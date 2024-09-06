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
	"reflect"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
)

func TestGetBackendConfigMapping(t *testing.T) {
	t.Parallel()
	testNamespace := "test-namespace"

	testServiceName := "test-service"
	testBeConfigName1 := "backendconfig-1"
	testBeConfigName2 := "backendconfig-2"
	backendConfigs := map[types.NamespacedName]*backendconfigv1.BackendConfig{
		{Namespace: testNamespace, Name: testBeConfigName1}: {},
		{Namespace: testNamespace, Name: testBeConfigName2}: {},
	}
	expectedServices := serviceNames{
		{Namespace: testNamespace, Name: testServiceName},
	}

	testCases := []struct {
		desc             string
		services         map[types.NamespacedName]*apiv1.Service
		expectedServices serviceNames
	}{
		{
			desc: "Specify BackendConfig with cloud.google.com/backend-config annotation, using the same BackendConfig for all ports",
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      testServiceName,
						Namespace: testNamespace,
						Annotations: map[string]string{
							backendConfigKey: `{"default":"backendconfig-1"}`,
						},
					},
				},
			},
			expectedServices: expectedServices,
		},
		{
			desc: "Specify BackendConfig with beta.cloud.google.com/backend-config annotation, using the same BackendConfig for all ports",
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      testServiceName,
						Namespace: testNamespace,
						Annotations: map[string]string{
							betaBackendConfigKey: `{"default":"backendconfig-1"}`,
						},
					},
				},
			},
			expectedServices: expectedServices,
		},
		{
			desc: "Specify BackendConfig with both cloud.google.com/backend-config and beta.cloud.google.com/backend-config annotation, using the same BackendConfig for all ports, cloud.google.com/backend-config should have precedence over the beta one",
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      testServiceName,
						Namespace: testNamespace,
						Annotations: map[string]string{
							backendConfigKey:     `{"default":"backendconfig-1"}`,
							betaBackendConfigKey: `{"ports": {"port1": "backendconfig-1", "port2": "backendconfig-2"}}`,
						},
					},
				},
			},
			expectedServices: expectedServices,
		},
		{
			desc: "Specify BackendConfig with cloud.google.com/backend-config annotation, using different BackendConfigs for each port, service will be associated with the BackendConfig for the alphabetically smallest port",
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      testServiceName,
						Namespace: testNamespace,
						Annotations: map[string]string{
							backendConfigKey: `{"ports": {"port1": "backendconfig-1", "port2": "backendconfig-2"}}`,
						},
					},
				},
			},
			expectedServices: expectedServices,
		},
		{
			desc: "Specify BackendConfig with beta.cloud.google.com/backend-config annotation, using different BackendConfigs for each port, service will be associated with the BackendConfig for the alphabetically smallest port",
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      testServiceName,
						Namespace: testNamespace,
						Annotations: map[string]string{
							betaBackendConfigKey: `{"ports": {"port1": "backendconfig-1", "port2": "backendconfig-2"}}`,
						},
					},
				},
			},
			expectedServices: expectedServices,
		},
		{
			desc: "Specify BackendConfig with both cloud.google.com/backend-config and beta.cloud.google.com/backend-config annotation, using different BackendConfigs for each port, service will be associated with the BackendConfig for the alphabetically smallest port, cloud.google.com/backend-config should have precedence over the beta one",
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: testNamespace, Name: testServiceName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      testServiceName,
						Namespace: testNamespace,
						Annotations: map[string]string{
							backendConfigKey:     `{"ports": {"port1": "backendconfig-1", "port2": "backendconfig-2"}}`,
							betaBackendConfigKey: `{"default":"backendconfig-1"}`,
						},
					},
				},
			},
			expectedServices: expectedServices,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			provider := NewProvider(&i2gw.ProviderConf{})
			gceProvider := provider.(*Provider)
			gceProvider.storage = newResourcesStorage()
			gceProvider.storage.Services = tc.services
			gceProvider.storage.BackendConfigs = backendConfigs

			beConfigToSvcs := getBackendConfigMapping(context.TODO(), gceProvider.storage)
			backendConfigKey := types.NamespacedName{Namespace: testNamespace, Name: testBeConfigName1}
			gotServiceList := beConfigToSvcs[backendConfigKey]
			if len(gotServiceList) != len(tc.expectedServices) {
				t.Errorf("Got BackendConfig mapped to %d services, expected %d", len(gotServiceList), len(tc.expectedServices))
			}
			if !reflect.DeepEqual(gotServiceList, tc.expectedServices) {
				t.Errorf("Got BackendConfig mapped to %v, expected %v", gotServiceList, tc.expectedServices)
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
