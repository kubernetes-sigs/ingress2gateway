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

package gce_emitter

import (
	gkegatewayv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

var (
	GCPBackendPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "GCPBackendPolicy",
	}

	GCPGatewayPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "GCPGatewayPolicy",
	}

	HealthCheckPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "HealthCheckPolicy",
	}

	GatewayClassNameFlag = "gateway-class-name"
)


func init() {
	i2gw.EmitterConstructorByName["gce"] = NewEmitter
	i2gw.RegisterProviderSpecificFlag("gce", i2gw.ProviderSpecificFlag{
		Name:         GatewayClassNameFlag,
		Description:  "The name of the GatewayClass to use for the Gateway",
		DefaultValue: "gke-l7-global-external-managed",
	})
}

type Emitter struct {
	conf *i2gw.EmitterConf
}

func NewEmitter(conf *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{conf: conf}
}

func (c *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	buildGceGatewayExtensions(ir, &gatewayResources)
	buildGceServiceExtensions(ir, &gatewayResources)
	c.updateGatewayClass(&gatewayResources)
	return gatewayResources, nil
}

func (c *Emitter) updateGatewayClass(gatewayResources *i2gw.GatewayResources) {
	gatewayClassName := "gke-l7-global-external-managed"
	if c.conf != nil && c.conf.ProviderSpecificFlags != nil {
		if flags, ok := c.conf.ProviderSpecificFlags["gce"]; ok {
			if val, ok := flags[GatewayClassNameFlag]; ok && val != "" {
				gatewayClassName = val
			}
		}
	}
	for i, gw := range gatewayResources.Gateways {
		gw.Spec.GatewayClassName = gatewayv1.ObjectName(gatewayClassName)
		gatewayResources.Gateways[i] = gw
	}
}

func buildGceGatewayExtensions(ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) {
	for gwyKey, gatewayContext := range ir.Gateways {
		gwyPolicy := addGatewayPolicyIfConfigured(gwyKey, &gatewayContext)
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

func addGatewayPolicyIfConfigured(gatewayNamespacedName types.NamespacedName, gatewayIR *emitterir.GatewayContext) *gkegatewayv1.GCPGatewayPolicy {
	if gatewayIR == nil || gatewayIR.Gce == nil {
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
		gcpGatewayPolicy.Spec.Default.SslPolicy = BuildGCPGatewayPolicySecurityPolicyConfig(gatewayIR)
	}
	return &gcpGatewayPolicy
}

func buildGceServiceExtensions(ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) {
	for svcKey, gceServiceIR := range ir.GceServices {
		bePolicy := addGCPBackendPolicyIfConfigured(svcKey, gceServiceIR)
		if bePolicy != nil {
			obj, err := i2gw.CastToUnstructured(bePolicy)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast GCPBackendPolicy to unstructured", bePolicy)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}

		hcPolicy := addHealthCheckPolicyIfConfigured(svcKey, &gceServiceIR)
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

func addGCPBackendPolicyIfConfigured(serviceNamespacedName types.NamespacedName, gceServiceIR gce.ServiceIR) *gkegatewayv1.GCPBackendPolicy {
	// If there is no specification related to GCPBackendPolicy feature, return nil.
	if gceServiceIR.SessionAffinity == nil && gceServiceIR.SecurityPolicy == nil {
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

	if gceServiceIR.SessionAffinity != nil {
		gcpBackendPolicy.Spec.Default.SessionAffinity = BuildGCPBackendPolicySessionAffinityConfig(gceServiceIR)
	}
	if gceServiceIR.SecurityPolicy != nil {
		gcpBackendPolicy.Spec.Default.SecurityPolicy = BuildGCPBackendPolicySecurityPolicyConfig(gceServiceIR)
	}

	return &gcpBackendPolicy
}

func addHealthCheckPolicyIfConfigured(serviceNamespacedName types.NamespacedName, gceServiceIR *gce.ServiceIR) *gkegatewayv1.HealthCheckPolicy {
	if gceServiceIR == nil {
		return nil
	}
	// If there is no specification related to HealthCheckPolicy feature, return nil.
	if gceServiceIR.HealthCheck == nil {
		return nil
	}

	healthCheckPolicy := gkegatewayv1.HealthCheckPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceNamespacedName.Namespace,
			Name:      serviceNamespacedName.Name,
		},
		Spec: gkegatewayv1.HealthCheckPolicySpec{
			Default: BuildHealthCheckPolicyConfig(gceServiceIR),
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
