/*
Copyright 2026 The Kubernetes Authors.

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

	"github.com/olekukonko/tablewriter"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Report collects user-facing notifications during a single conversion run. A nil *Report is safe
// to use - calls to Add and Notifier become no-ops.
type Report struct {
	mu            sync.Mutex
	notifications map[string][]Notification
}

// NewReport creates a new Report and returns a pointer to it.
func NewReport() *Report {
	return &Report{
		notifications: make(map[string][]Notification),
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

// Render returns human-readable tables grouped by source. Called once after conversion completes.
// Returns nil when r is nil.
func (r *Report) Render() map[string]string {
	if r == nil {
		return nil
	}

	out := make(map[string]string)

	r.mu.Lock()
	defer r.mu.Unlock()

	for source, notifications := range r.notifications {
		table := strings.Builder{}

		t := tablewriter.NewWriter(&table)
		t.SetHeader([]string{"Message Type", "Notification", "Calling Object"})
		t.SetColWidth(200)
		t.SetRowLine(true)

		for _, n := range notifications {
			row := []string{string(n.Type), n.Message, convertObjectsToStr(n.CallingObjects)}
			t.Append(row)
		}

		fmt.Fprintf(&table, "Notifications from %s:\n", strings.ToUpper(source))
		t.Render()
		out[source] = table.String()
	}

	return out
}

// NotifyFunc is the signature for a scoped notification callback. Used by providers and emitters
// to record user-facing information related to conversions.
type NotifyFunc func(MessageType, string, ...client.Object)

// NoopNotify is a no-op NotifyFunc used when no Report is configured (typically in tests).
func NoopNotify(_ MessageType, _ string, _ ...client.Object) {}
