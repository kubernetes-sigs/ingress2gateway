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

package notifications

import (
	"fmt"
	"strings"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/olekukonko/tablewriter"
)

func init() {
	NotificationAggr = BuildNotificationAggregator()
}

const (
	InfoNotification    MessageType = "INFO"
	WarningNotification MessageType = "WARNING"
	ErrorNotification   MessageType = "ERROR"
)

type MessageType string

type Notification struct {
	Type           MessageType
	Message        string
	CallingObjects []client.Object
}

type NotificationAggregator struct {
	mutex         sync.Mutex
	Notifications map[string][]Notification
}

var NotificationAggr NotificationAggregator

// NotificationCallback is a callback function used to send notifications from within the common
// package without the common package having knowledge about which provider is making a call it
type NotificationCallback func(mType MessageType, message string, CallingObjects ...client.Object)

// BuildNotificationAggregator returns an instance of initialized NotificationAggregator
func BuildNotificationAggregator() NotificationAggregator {
	return NotificationAggregator{Notifications: map[string][]Notification{}}
}

// DispatchNotification is used to send a notification to the NotificationAggregator
func (na *NotificationAggregator) DispatchNotification(notification Notification, ProviderName string) {
	na.mutex.Lock()
	na.Notifications[ProviderName] = append(na.Notifications[ProviderName], notification)
	na.mutex.Unlock()
}

// CreateNotificationTables takes all generated notifications and returns a map[string]string
// that displays the notifications in a tabular format based on provider
func (na *NotificationAggregator) CreateNotificationTables() map[string]string {
	notificationTablesMap := make(map[string]string)

	for provider, msgs := range na.Notifications {
		providerTable := strings.Builder{}

		t := tablewriter.NewWriter(&providerTable)
		t.SetHeader([]string{"Message Type", "Notification", "Calling Object"})
		t.SetColWidth(200)
		t.SetRowLine(true)

		for _, n := range msgs {
			row := []string{string(n.Type), n.Message, convertObjectsToStr(n.CallingObjects)}
			t.Append(row)
		}

		providerTable.WriteString(fmt.Sprintf("Notifications from %v:\n", strings.ToUpper(provider)))
		t.Render()
		notificationTablesMap[provider] = providerTable.String()
	}

	return notificationTablesMap
}

// convertObjectsToStr takes a slice of client.Object as input and extracts the Kind and Namespaced Name
func convertObjectsToStr(ob []client.Object) string {
	var sb strings.Builder

	for i, o := range ob {
		if i > 0 {
			sb.WriteString(", ")
		}
		object := o.GetObjectKind().GroupVersionKind().Kind + ": " + client.ObjectKeyFromObject(o).String()
		sb.WriteString(object)
	}

	return sb.String()
}

func NewNotification(mType MessageType, message string, callingObjects ...client.Object) Notification {
	return Notification{Type: mType, Message: message, CallingObjects: callingObjects}
}
