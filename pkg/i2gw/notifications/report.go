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

package notifications

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ANSI color codes for terminal output.
const (
	colorReset        = "\033[0m"
	colorRed          = "\033[31m"
	colorYellow       = "\033[33m"
	colorGray         = "\033[90m"
	colorCyan         = "\033[36m"
	colorBrightGreen  = "\033[92m"
	colorBrightPurple = "\033[95m"
)

// Number of dash characters in the top border after the level label.
const boxDashes = 40

// Report collects user-facing notifications during a single conversion run. A nil *Report is safe
// to use - calls to Add and Notifier become no-ops.
type Report struct {
	mu            sync.Mutex
	notifications map[string][]Notification
	noColor       bool
}

// NewReport creates a new Report and returns a pointer to it. Set noColor to true to disable ANSI
// color codes in the output.
func NewReport(noColor bool) *Report {
	return &Report{
		notifications: make(map[string][]Notification),
		noColor:       noColor,
	}
}

// Add records a notification under the given source name.
func (r *Report) Add(source string, n Notification) {
	if r == nil {
		return
	}

	r.mu.Lock()
	r.notifications[source] = append(r.notifications[source], n)
	r.mu.Unlock()
}

// Notifier returns a convenience function scoped to a single source name, eliminating the need for
// per-package boilerplate.
func (r *Report) Notifier(source string) NotifyFunc {
	return func(mt MessageType, msg string, objs ...client.Object) {
		r.Add(source, Notification{
			Type:           mt,
			Message:        msg,
			CallingObjects: objs,
		})
	}
}

// Render returns notifications as a single human-readable string of colored boxes. Called once
// after conversion completes. Notifications are sorted by source name for deterministic output.
// Returns "" when r is nil or there are no notifications.
func (r *Report) Render() string {
	if r == nil {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	sources := slices.Sorted(maps.Keys(r.notifications))

	// Returns the ANSI code, or "" when color is disabled.
	c := func(code string) string {
		if r.noColor {
			return ""
		}
		return code
	}

	var buf strings.Builder

	for _, source := range sources {
		for _, n := range r.notifications[source] {
			label, lcolor := levelLabel(n.Type)

			// Top border with level.
			fmt.Fprintf(&buf, "%s┌─ %s%s%s %s%s\n",
				c(colorGray), c(lcolor), label, c(colorGray),
				strings.Repeat("─", boxDashes), c(colorReset))

			// Message.
			fmt.Fprintf(&buf, "%s│%s  %s\n",
				c(colorGray), c(colorReset), n.Message)

			// Source attribute.
			fmt.Fprintf(&buf, "%s│%s  %ssource:%s %s%s%s\n",
				c(colorGray), c(colorReset),
				c(colorGray), c(colorReset),
				c(colorBrightPurple), strings.ToUpper(source), c(colorReset))

			// Calling objects.
			if len(n.CallingObjects) > 0 {
				key := "object"
				if len(n.CallingObjects) > 1 {
					key = "objects"
				}
				fmt.Fprintf(&buf, "%s│%s  %s%s:%s %s%s%s\n",
					c(colorGray), c(colorReset),
					c(colorGray), key, c(colorReset),
					c(colorBrightGreen), objectsToStr(n.CallingObjects), c(colorReset))
			}

			// Bottom border.
			fmt.Fprintf(&buf, "%s└─%s\n",
				c(colorGray), c(colorReset))
		}
	}

	return buf.String()
}

// levelLabel returns a display label and its ANSI color for the given MessageType. Labels for
// WARN and INFO include a trailing space so all labels are 5 characters wide.
func levelLabel(mt MessageType) (string, string) {
	switch mt {
	case ErrorNotification:
		return "ERROR", colorRed
	case WarningNotification:
		return "WARN ", colorYellow
	case InfoNotification:
		return "INFO ", colorCyan
	default:
		return "INFO ", colorCyan
	}
}

// NotifyFunc is the signature for a scoped notification callback. Used by providers and emitters
// to record user-facing information related to conversions.
type NotifyFunc func(MessageType, string, ...client.Object)

// NoopNotify is a no-op NotifyFunc used when no Report is configured (typically in tests).
func NoopNotify(_ MessageType, _ string, _ ...client.Object) {}
