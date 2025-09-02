package common

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CoreGroup   = "core"
	SecretKind  = "Secret"
	ServiceKind = "Service"
)

// NotificationCollector is an interface for collecting notifications from filter creation
type NotificationCollector interface {
	// Add adds a notification to the collection
	Add(messageType notifications.MessageType, message string, obj client.Object)
	// AddInfo is a convenience method for adding info notifications
	AddInfo(message string, obj client.Object)
	// AddWarning is a convenience method for adding warning notifications
	AddWarning(message string, obj client.Object)
	// AddError is a convenience method for adding error notifications
	AddError(message string, obj client.Object)
	// GetNotifications returns all collected notifications
	GetNotifications() []notifications.Notification
}

// SliceNotificationCollector collects notifications in a slice (for CRDs)
type SliceNotificationCollector struct {
	notifications []notifications.Notification
}

// NewSliceNotificationCollector creates a new slice-based notification collector
func NewSliceNotificationCollector() *SliceNotificationCollector {
	return &SliceNotificationCollector{
		notifications: make([]notifications.Notification, 0),
	}
}

// Add adds a notification to the slice
func (c *SliceNotificationCollector) Add(messageType notifications.MessageType, message string, obj client.Object) {
	notification := notifications.NewNotification(messageType, message, obj)
	c.notifications = append(c.notifications, notification)
}

// AddInfo adds an info notification
func (c *SliceNotificationCollector) AddInfo(message string, obj client.Object) {
	c.Add(notifications.InfoNotification, message, obj)
}

// AddWarning adds a warning notification
func (c *SliceNotificationCollector) AddWarning(message string, obj client.Object) {
	c.Add(notifications.WarningNotification, message, obj)
}

// AddError adds an error notification
func (c *SliceNotificationCollector) AddError(message string, obj client.Object) {
	c.Add(notifications.ErrorNotification, message, obj)
}

// GetNotifications returns all collected notifications
func (c *SliceNotificationCollector) GetNotifications() []notifications.Notification {
	return c.notifications
}

// DispatchNotificationCollector dispatches notifications immediately (for annotations)
type DispatchNotificationCollector struct{}

// NewDispatchNotificationCollector creates a new dispatch-based notification collector
func NewDispatchNotificationCollector() *DispatchNotificationCollector {
	return &DispatchNotificationCollector{}
}

// Add dispatches a notification immediately
func (c *DispatchNotificationCollector) Add(messageType notifications.MessageType, message string, obj client.Object) {
	notification := notifications.NewNotification(messageType, message, obj)
	notifications.NotificationAggr.DispatchNotification(notification, "nginx")
}

// AddInfo dispatches an info notification immediately
func (c *DispatchNotificationCollector) AddInfo(message string, obj client.Object) {
	c.Add(notifications.InfoNotification, message, obj)
}

// AddWarning dispatches a warning notification immediately
func (c *DispatchNotificationCollector) AddWarning(message string, obj client.Object) {
	c.Add(notifications.WarningNotification, message, obj)
}

// AddError dispatches an error notification immediately
func (c *DispatchNotificationCollector) AddError(message string, obj client.Object) {
	c.Add(notifications.ErrorNotification, message, obj)
}

// GetNotifications returns empty slice since notifications are dispatched immediately
func (c *DispatchNotificationCollector) GetNotifications() []notifications.Notification {
	return []notifications.Notification{}
}
