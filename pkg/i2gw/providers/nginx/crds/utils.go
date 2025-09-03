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

package crds

import (
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	nginxv1 "github.com/nginx/kubernetes-ingress/pkg/apis/configuration/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Utility functions

// Ptr is Generic pointer conversion utility
func Ptr[T any](t T) *T {
	return &t
}

// findUpstream finds an upstream by name in the upstreams slice
func findUpstream(upstreams []nginxv1.Upstream, name string) *nginxv1.Upstream {
	for _, upstream := range upstreams {
		if upstream.Name == name {
			return &upstream
		}
	}
	return nil
}

// sanitizeHostname is implemented in gateway_builder.go

// containsRegexPatterns checks if a value contains regex special characters
func containsRegexPatterns(s string) bool {
	return strings.ContainsAny(s, `\.+*?^$()[]{}|`)
}

// addNotification adds a notification to the notification list
func addNotification(notificationList *[]notifications.Notification, messageType notifications.MessageType, message string, obj client.Object) {
	n := notifications.NewNotification(messageType, message, obj)
	*notificationList = append(*notificationList, n)
}

// getFirstGateway returns the first gateway from a map of gateways
func getFirstGateway(gateways map[types.NamespacedName]intermediate.GatewayContext) intermediate.GatewayContext {
	for _, gateway := range gateways {
		return gateway
	}
	return intermediate.GatewayContext{}
}
