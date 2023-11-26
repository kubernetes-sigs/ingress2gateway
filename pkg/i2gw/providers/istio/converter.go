/*
Copyright 2023 The Kubernetes Authors.

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

package istio

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type converter struct{}

func newConverter() converter {
	return converter{}
}

func (c *converter) convert(storage storage) (i2gw.GatewayResources, field.ErrorList) {
	// TODO(#100): This logic only do a simple metadata conversion. Need to implement istio conversion logic.
	gatewayResources := i2gw.GatewayResources{
		Gateways: make(map[types.NamespacedName]gatewayv1beta1.Gateway),
	}

	for namespacedName, gw := range storage.Gateways {
		apiVersion, kind := common.GatewayGVK.ToAPIVersionAndKind()

		gatewayResources.Gateways[namespacedName] = gatewayv1beta1.Gateway{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       gw.Namespace,
				Name:            gw.Name,
				Labels:          gw.Labels,
				Annotations:     gw.Annotations,
				OwnerReferences: gw.OwnerReferences,
				Finalizers:      gw.Finalizers,
			},
		}
	}

	return gatewayResources, field.ErrorList{field.Forbidden(field.NewPath(""), "conversion is WIP")}
}
