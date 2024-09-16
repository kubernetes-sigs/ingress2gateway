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
	buildGceServiceExtensions(ir, &gatewayResources)
	return gatewayResources, nil
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
	if serviceIR.Gce.SecurityPolicy != nil {
		backendPolicy.Spec.Default.SecurityPolicy = extensions.BuildBackendPolicySecurityPolicyConfig(serviceIR)
	}

	return &backendPolicy
}
