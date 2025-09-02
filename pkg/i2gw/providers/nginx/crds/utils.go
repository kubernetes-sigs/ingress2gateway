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
	"fmt"
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

// Notification system utilities

// NotificationCollector provides a unified way to collect notifications
type NotificationCollector struct {
	notifications []notifications.Notification
}

// NewNotificationCollector creates a new notification collector
func NewNotificationCollector() *NotificationCollector {
	return &NotificationCollector{
		notifications: make([]notifications.Notification, 0),
	}
}

// AddInfo adds an info notification
func (nc *NotificationCollector) AddInfo(message string, obj client.Object) {
	nc.addNotification(notifications.InfoNotification, message, obj)
}

// AddWarning adds a warning notification
func (nc *NotificationCollector) AddWarning(message string, obj client.Object) {
	nc.addNotification(notifications.WarningNotification, message, obj)
}

// AddError adds an error notification
func (nc *NotificationCollector) AddError(message string, obj client.Object) {
	nc.addNotification(notifications.ErrorNotification, message, obj)
}

// AddInfof adds a formatted info notification
func (nc *NotificationCollector) AddInfof(format string, args ...interface{}) {
	nc.addNotification(notifications.InfoNotification, fmt.Sprintf(format, args...), nil)
}

// AddWarningf adds a formatted warning notification
func (nc *NotificationCollector) AddWarningf(format string, args ...interface{}) {
	nc.addNotification(notifications.WarningNotification, fmt.Sprintf(format, args...), nil)
}

// AddErrorf adds a formatted error notification
func (nc *NotificationCollector) AddErrorf(format string, args ...interface{}) {
	nc.addNotification(notifications.ErrorNotification, fmt.Sprintf(format, args...), nil)
}

// GetNotifications returns all collected notifications
func (nc *NotificationCollector) GetNotifications() []notifications.Notification {
	return nc.notifications
}

// Clear removes all notifications
func (nc *NotificationCollector) Clear() {
	nc.notifications = nc.notifications[:0]
}

// Merge adds notifications from another collector
func (nc *NotificationCollector) Merge(other *NotificationCollector) {
	nc.notifications = append(nc.notifications, other.notifications...)
}

// addNotification is the internal method for adding notifications
func (nc *NotificationCollector) addNotification(messageType notifications.MessageType, message string, obj client.Object) {
	n := notifications.NewNotification(messageType, message, obj)
	nc.notifications = append(nc.notifications, n)
}

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
