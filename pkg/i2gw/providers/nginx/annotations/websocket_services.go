/*
Copyright 2025 The Kubernetes Authors.

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

package annotations

import (
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

func WebSocketServicesFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, _ *provider_intermediate.IR) field.ErrorList {
	for _, ingress := range ingresses {
		if webSocketServices, exists := ingress.Annotations[nginxWebSocketServicesAnnotation]; exists && webSocketServices != "" {
			message := "nginx.org/websocket-services: Please make sure the services are configured to support WebSocket connections. This annotation does not create any Gateway API resources."
			notify(notifications.InfoNotification, message, &ingress)
		}
	}

	return nil
}
