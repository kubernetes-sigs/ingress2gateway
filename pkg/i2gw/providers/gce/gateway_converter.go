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
	gkegatewayv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/gce/extensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type irToGatewayResourcesConverter struct{}

// newIRToGatewayResourcesConverter returns an gce irToGatewayResourcesConverter instance.
func newIRToGatewayResourcesConverter() irToGatewayResourcesConverter {
	return irToGatewayResourcesConverter{}
}

func (c *irToGatewayResourcesConverter) irToGateway(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := common.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	buildGceGatewayExtensions(ir, &gatewayResources)
	buildGceServiceExtensions(ir, &gatewayResources)
	return gatewayResources, nil
}

func buildGceGatewayExtensions(ir intermediate.IR, gatewayResources *i2gw.GatewayResources) {
	for gwyKey, gatewayContext := range ir.Gateways {
		gwyPolicy := addGatewayPolicyIfConfigured(gwyKey, gatewayContext.ProviderSpecificIR)
		if gwyPolicy == nil {
			continue
		}
		obj, err := i2gw.CastToUnstructured(gwyPolicy)
		if err != nil {
			notify(notifications.ErrorNotification, "Failed to cast GCPGatewayPolicy to unstructured", gwyPolicy)
			continue
		}
		gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
	}
}

func addGatewayPolicyIfConfigured(gatewayNamespacedName types.NamespacedName, gatewayIR intermediate.ProviderSpecificGatewayIR) *gkegatewayv1.GCPGatewayPolicy {
	if gatewayIR.Gce == nil {
		return nil
	}
	// If there is no specification related to GCPGatewayPolicy feature, return nil.
	if gatewayIR.Gce.SslPolicy == nil {
		return nil
	}
	gcpGatewayPolicy := gkegatewayv1.GCPGatewayPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: gatewayNamespacedName.Namespace,
			Name:      gatewayNamespacedName.Name,
		},
		Spec: gkegatewayv1.GCPGatewayPolicySpec{
			Default: &gkegatewayv1.GCPGatewayPolicyConfig{},
			TargetRef: gatewayv1alpha2.NamespacedPolicyTargetReference{
				Group: "gateway.networking.k8s.io",
				Kind:  "Gateway",
				Name:  gatewayv1.ObjectName(gatewayNamespacedName.Name),
			},
		},
	}
	gcpGatewayPolicy.SetGroupVersionKind(GCPGatewayPolicyGVK)
	if gatewayIR.Gce.SslPolicy != nil {
		gcpGatewayPolicy.Spec.Default.SslPolicy = extensions.BuildGCPGatewayPolicySecurityPolicyConfig(gatewayIR)
	}
	return &gcpGatewayPolicy
}

func buildGceServiceExtensions(ir intermediate.IR, gatewayResources *i2gw.GatewayResources) {
	for svcKey, serviceIR := range ir.Services {
		bePolicy := addGCPBackendPolicyIfConfigured(svcKey, serviceIR)
		if bePolicy != nil {
			obj, err := i2gw.CastToUnstructured(bePolicy)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast GCPBackendPolicy to unstructured", bePolicy)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}

		hcPolicy := addHealthCheckPolicyIfConfigured(svcKey, serviceIR)
		if hcPolicy != nil {
			obj, err := i2gw.CastToUnstructured(hcPolicy)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast HealthCheckPolicy to unstructured", hcPolicy)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}
	}
}

func addGCPBackendPolicyIfConfigured(serviceNamespacedName types.NamespacedName, serviceIR intermediate.ProviderSpecificServiceIR) *gkegatewayv1.GCPBackendPolicy {
	if serviceIR.Gce == nil {
		return nil
	}
	// If there is no specification related to GCPBackendPolicy feature, return nil.
	if serviceIR.Gce.SessionAffinity == nil && serviceIR.Gce.SecurityPolicy == nil {
		return nil
	}

	gcpBackendPolicy := gkegatewayv1.GCPBackendPolicy{
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
	gcpBackendPolicy.SetGroupVersionKind(GCPBackendPolicyGVK)

	if serviceIR.Gce.SessionAffinity != nil {
		gcpBackendPolicy.Spec.Default.SessionAffinity = extensions.BuildGCPBackendPolicySessionAffinityConfig(serviceIR)
	}
	if serviceIR.Gce.SecurityPolicy != nil {
		gcpBackendPolicy.Spec.Default.SecurityPolicy = extensions.BuildGCPBackendPolicySecurityPolicyConfig(serviceIR)
	}

	return &gcpBackendPolicy
}

func addHealthCheckPolicyIfConfigured(serviceNamespacedName types.NamespacedName, serviceIR intermediate.ProviderSpecificServiceIR) *gkegatewayv1.HealthCheckPolicy {
	if serviceIR.Gce == nil {
		return nil
	}
	// If there is no specification related to HealthCheckPolicy feature, return nil.
	if serviceIR.Gce.HealthCheck == nil {
		return nil
	}

	healthCheckPolicy := gkegatewayv1.HealthCheckPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceNamespacedName.Namespace,
			Name:      serviceNamespacedName.Name,
		},
		Spec: gkegatewayv1.HealthCheckPolicySpec{
			Default: extensions.BuildHealthCheckPolicyConfig(serviceIR),
			TargetRef: gatewayv1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  "Service",
				Name:  gatewayv1.ObjectName(serviceNamespacedName.Name),
			},
		},
	}
	healthCheckPolicy.SetGroupVersionKind(HealthCheckPolicyGVK)
	return &healthCheckPolicy
}
