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
	"encoding/json"

	gkegatewayv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/gce/extensions"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type serviceNames []types.NamespacedName

func buildGceServiceIR(ctx context.Context, storage *storage, ir *intermediate.IR) {
	if ir.Services == nil {
		ir.Services = make(map[types.NamespacedName]intermediate.ProviderSpecificServiceIR)
	}

	beConfigToSvcs := getBackendConfigMapping(ctx, storage)
	for beConfigKey, beConfig := range storage.BackendConfigs {
		if beConfig == nil {
			continue
		}
		gceServiceIR := beConfigToGceServiceIR(beConfig)
		services := beConfigToSvcs[beConfigKey]
		for _, svcKey := range services {
			serviceIR := ir.Services[svcKey]
			serviceIR.Gce = &gceServiceIR
			ir.Services[svcKey] = serviceIR
		}
	}
}

func getBackendConfigMapping(ctx context.Context, storage *storage) map[types.NamespacedName]serviceNames {
	beConfigToSvcs := make(map[types.NamespacedName]serviceNames)

	for _, service := range storage.Services {
		svc := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
		ctx = context.WithValue(ctx, serviceKey, service)

		// Read BackendConfig based on v1 BackendConfigKey.
		beConfigName, exists := getBackendConfigName(ctx, service, backendConfigKey)
		if exists {
			beConfigKey := types.NamespacedName{Namespace: service.Namespace, Name: beConfigName}
			beConfigToSvcs[beConfigKey] = append(beConfigToSvcs[beConfigKey], svc)
			continue
		}

		// Read BackendConfig based on v1beta1 BackendConfigKey.
		beConfigName, exists = getBackendConfigName(ctx, service, betaBackendConfigKey)
		if exists {
			beConfigKey := types.NamespacedName{Namespace: service.Namespace, Name: beConfigName}
			beConfigToSvcs[beConfigKey] = append(beConfigToSvcs[beConfigKey], svc)
			continue
		}
	}
	return beConfigToSvcs
}

// Get names of the BackendConfig in the cluster based on the BackendConfig
// annotation on k8s Services.
func getBackendConfigName(ctx context.Context, service *apiv1.Service, backendConfigKey string) (string, bool) {
	val, exists := getBackendConfigAnnotation(service, backendConfigKey)
	if !exists {
		return "", false
	}

	return parseBackendConfigName(ctx, val)
}

// Get the backend config annotation from the K8s service if it exists.
func getBackendConfigAnnotation(service *apiv1.Service, backendConfigKey string) (string, bool) {
	val, ok := service.Annotations[backendConfigKey]
	if ok {
		return val, ok
	}
	return "", false
}

type backendConfigs struct {
	Default string            `json:"default,omitempty"`
	Ports   map[string]string `json:"ports,omitempty"`
}

// Parse the name of the BackendConfig based on the annotation.
// If different BackendConfigs are used on the same service, pick the one with
// the alphabetically smallest name.
func parseBackendConfigName(ctx context.Context, val string) (string, bool) {
	service := ctx.Value(serviceKey).(*apiv1.Service)

	var configs backendConfigs
	if err := json.Unmarshal([]byte(val), &configs); err != nil {
		notify(notifications.ErrorNotification, "BackendConfig annotation is invalid json", service)
		return "", false
	}

	if configs.Default == "" && len(configs.Ports) == 0 {
		notify(notifications.ErrorNotification, "No BackendConfig's found in annotation", service)
		return "", false
	}

	if len(configs.Ports) != 0 {
		notify(notifications.ErrorNotification, "HealthCheckPolicy and GCPBackendPolicy can only be attached on the whole service, so having a dedicate policy for each port is not yet supported. Picking the first BackendConfig to translate to corresponding Gateway policy.", service)
		// Return the BackendConfig associated with the alphabetically smallest port.
		var backendConfigName string
		var lowestPort string
		for p, name := range configs.Ports {
			if lowestPort == "" || p < lowestPort {
				backendConfigName = name
				lowestPort = p
			}
		}
		return backendConfigName, true
	}
	return configs.Default, true
}

func beConfigToGceServiceIR(beConfig *backendconfigv1.BackendConfig) intermediate.GceServiceIR {
	var gceServiceIR intermediate.GceServiceIR
	if beConfig.Spec.SessionAffinity != nil {
		gceServiceIR.SessionAffinity = extensions.BuildIRSessionAffinityConfig(beConfig)
	}

	return gceServiceIR
}

func buildGceServiceExtensions(ir intermediate.IR, gatewayResources *i2gw.GatewayResources) {
	for svcKey, serviceIR := range ir.Services {
		bePolicy := addBackendPolicyIfConfigured(svcKey, serviceIR)
		if bePolicy == nil {
			continue
		}
		obj, err := i2gw.CastToUnstructured(bePolicy)
		if err != nil {
			notify(notifications.ErrorNotification, "Failed to cast GCPBackendPolicy to unstructured", bePolicy)
			continue
		}
		gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
	}
}

func addBackendPolicyIfConfigured(serviceNamespacedName types.NamespacedName, serviceIR intermediate.ProviderSpecificServiceIR) *gkegatewayv1.GCPBackendPolicy {
	if serviceIR.Gce == nil {
		return nil
	}
	backendPolicy := gkegatewayv1.GCPBackendPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceNamespacedName.Namespace,
			Name:      serviceNamespacedName.Name,
		},
		Spec: gkegatewayv1.GCPBackendPolicySpec{
			Default: &gkegatewayv1.GCPBackendPolicyConfig{},
			TargetRef: gatewayv1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  "Service",
				Name:  gatewayv1.ObjectName(serviceNamespacedName.Name),
			},
		},
	}
	backendPolicy.SetGroupVersionKind(GCPBackendPolicyGVK)

	if serviceIR.Gce.SessionAffinity != nil {
		backendPolicy.Spec.Default.SessionAffinity = extensions.BuildBackendPolicySessionAffinityConfig(serviceIR)
	}

	return &backendPolicy
}
