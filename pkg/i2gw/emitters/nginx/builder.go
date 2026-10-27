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

package nginx

import (
	"strings"

	ngfv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type BuilderMap struct {
	ClientSettingsPolicy map[types.NamespacedName]*ngfv1alpha1.ClientSettingsPolicy
}

func NewBuilderMap() *BuilderMap {
	return &BuilderMap{
		ClientSettingsPolicy: make(map[types.NamespacedName]*ngfv1alpha1.ClientSettingsPolicy),
	}
}

// convert K8s quantity back to NGINX size for spec
func toNginxSize(value string) string {
	switch {
	case strings.HasSuffix(value, "Ki"):
		return strings.TrimSuffix(value, "Ki") + "k"
	case strings.HasSuffix(value, "Mi"):
		return strings.TrimSuffix(value, "Mi") + "m"
	case strings.HasSuffix(value, "Gi"):
		return strings.TrimSuffix(value, "Gi") + "g"
	default:
		return value
	}
}

// EmitClientSettings creates an NGINX Gateway Fabric ClientSettingsPolicy targeting the specified HTTPRoute
func EmitClientSettings(name, namespace string, routeName string, maxSize string) *ngfv1alpha1.ClientSettingsPolicy {
	maxSize = toNginxSize(maxSize)
	maxSizeQuantity := ngfv1alpha1.Size(maxSize)

	return &ngfv1alpha1.ClientSettingsPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.nginx.org/v1alpha1",
			Kind:       "ClientSettingsPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ngfv1alpha1.ClientSettingsPolicySpec{
			TargetRef: gatewayv1.LocalPolicyTargetReference{
				Group: "gateway.networking.k8s.io",
				Kind:  "HTTPRoute",
				Name:  gatewayv1.ObjectName(routeName),
			},
			Body: &ngfv1alpha1.ClientBody{
				MaxSize: &maxSizeQuantity,
			},
		},
	}
}
