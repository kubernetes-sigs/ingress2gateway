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
	"fmt"

	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// setGCEGatewayClasses updates the list of Gateways to use GCE GatewayClass.
func setGCEGatewayClasses(ingresses []networkingv1.Ingress, gatewayContexts map[types.NamespacedName]providerir.GatewayContext, gatewayClassName string) field.ErrorList {
	var errs field.ErrorList

	// Since we already validated ingress resources when reading, there are
	// only two cases here:
	//   1. `kubernetes.io/ingress.class` exists in ingress annotation. In this
	//      case, we use it to map to the corresponding gateway class.
	//   2. Annotation does not exist. In this case, the ingress is defaulted
	//      to use the GCE external ingress implementation, and should be
	//      mapped to `gke-l7-global-external-managed`.
	for _, ingress := range ingresses {
		gwKey := types.NamespacedName{Namespace: ingress.Namespace, Name: common.GetIngressClass(ingress)}
		existingGateway := gatewayContexts[gwKey].Gateway

		newGateway, err := setGCEGatewayClass(ingress, existingGateway, gatewayClassName)
		if err != nil {
			errs = append(errs, err)
		}
		gatewayContexts[gwKey] = providerir.GatewayContext{Gateway: newGateway}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// setGCEGatewayClass sets the Gateway to the corresponding GCE GatewayClass.
func setGCEGatewayClass(ingress networkingv1.Ingress, gateway gatewayv1.Gateway, gatewayClassName string) (gatewayv1.Gateway, *field.Error) {
	if gatewayClassName != "" {
		gateway.Spec.GatewayClassName = gatewayv1.ObjectName(gatewayClassName)
		return gateway, nil
	}

	ingressClass := common.GetIngressClass(ingress)

	ingressName := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)
	// Get GCE GatewayClass from from GCE Ingress class.
	newGatewayClass, err := ingClassToGwyClassGCE(ingressClass)
	if err != nil {
		return gateway, field.NotSupported(field.NewPath(ingressName).Child("ObjectMeta", "Annotations", "kubernetes.io/ingress.class"), ingressClass, []string{gceIngressClass, gceL7ILBIngressClass})
	}
	gateway.Spec.GatewayClassName = gatewayv1.ObjectName(newGatewayClass)
	return gateway, nil
}

// ingClassToGwyClassGCE returns the corresponding GCE Gateway Class based on the
// given GCE ingress class.
func ingClassToGwyClassGCE(ingressClass string) (string, error) {
	switch ingressClass {
	case gceIngressClass:
		return gceL7GlobalExternalManagedGatewayClass, nil
	case gceL7ILBIngressClass:
		return gceL7RegionalInternalGatewayClass, nil
	case "":
		return gceL7GlobalExternalManagedGatewayClass, nil
	default:
		return "", fmt.Errorf("Given GCE Ingress Class not supported")
	}
}
