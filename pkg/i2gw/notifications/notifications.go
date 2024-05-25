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
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

func init() {
	CommonNotification = NotificationAggregator{make([]Notification, 0)}
}


const (
	InfoNotification    MessageType = "INFO"
	WarningNotification MessageType = "WARNING"
	ErrorNotification   MessageType = "ERROR"
)

type MessageType string
type Notification struct {
	Type     MessageType
	Message  string
	Provider string
}

type NotificationAggregator struct {
	Notifications []Notification
}

var CommonNotification NotificationAggregator

func (na *NotificationAggregator) DispatchNotication(notification Notification) {
	na.Notifications = append(na.Notifications, notification)
}

func (na *NotificationAggregator) ProcessNotifications() {
	// Create a mapping of provider and their messages
	providerNotifications := make(map[string][]Notification)

	// Segregate messages into
	for _, msg := range na.Notifications {
		providerNotifications[msg.Provider] = append(providerNotifications[msg.Provider], msg)
	}

	for provider, msgs := range providerNotifications {
		t := table.NewWriter()

		t.SetOutputMirror(os.Stdout)
		t.SetTitle(fmt.Sprintf("Notifications from %v", provider))
		t.AppendHeader(table.Row{"Provider", "Message Type", "Notification"})
		
		t.SetRowPainter(func(row table.Row) text.Colors {
			switch row[1]{
			case InfoNotification:
				return text.Colors{text.BgBlack, text.FgHiWhite}
			case WarningNotification:
				return text.Colors{text.BgBlack, text.FgHiBlue}
			case ErrorNotification:
				return text.Colors{text.BgBlack, text.FgRed}
			default:
				return text.Colors{text.BgBlack, text.FgWhite}
			}
		})

		for _, n := range msgs {
			t.AppendRow(table.Row{n.Provider, n.Type, n.Message})
		}

		t.SetStyle(table.StyleRounded)
		t.Render()
	}
}
