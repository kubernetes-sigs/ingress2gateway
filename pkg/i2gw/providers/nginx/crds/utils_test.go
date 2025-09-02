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
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	nginxv1 "github.com/nginx/kubernetes-ingress/pkg/apis/configuration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUtilityFunctions(t *testing.T) {
	t.Run("Ptr function", func(t *testing.T) {
		// Test with string
		str := "test"
		strPtr := Ptr(str)
		if strPtr == nil || *strPtr != str {
			t.Errorf("Ptr failed for string: expected %s, got %v", str, strPtr)
		}

		// Test with int
		num := 42
		numPtr := Ptr(num)
		if numPtr == nil || *numPtr != num {
			t.Errorf("Ptr failed for int: expected %d, got %v", num, numPtr)
		}
	})

	t.Run("findUpstream function", func(t *testing.T) {
		upstreams := []nginxv1.Upstream{
			{
				Name:    "web-backend",
				Service: "web-service",
				Port:    80,
			},
			{
				Name:    "api-backend",
				Service: "api-service",
				Port:    8080,
			},
		}

		// Test finding existing upstream
		found := findUpstream(upstreams, "api-backend")
		if found == nil {
			t.Error("Expected to find api-backend upstream")
		} else if found.Service != "api-service" || found.Port != 8080 {
			t.Errorf("Found wrong upstream: %+v", found)
		}

		// Test not finding upstream
		notFound := findUpstream(upstreams, "nonexistent")
		if notFound != nil {
			t.Errorf("Expected nil for nonexistent upstream, got %+v", notFound)
		}
	})

	t.Run("containsRegexPatterns function", func(t *testing.T) {
		tests := []struct {
			input    string
			expected bool
		}{
			{"simple", false},
			{"domain.com", true}, // contains dot
			{"Mozilla*", true},   // contains asterisk
			{"test?", true},      // contains question mark
			{"^start", true},     // contains caret
			{"end$", true},       // contains dollar
			{"normal-text", false},
			{"", false},
		}

		for _, test := range tests {
			result := containsRegexPatterns(test.input)
			if result != test.expected {
				t.Errorf("containsRegexPatterns(%q) = %v, expected %v", test.input, result, test.expected)
			}
		}
	})
}

func TestNotificationCollector(t *testing.T) {
	t.Run("basic notification collection", func(t *testing.T) {
		collector := NewNotificationCollector()

		vs := &nginxv1.VirtualServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vs",
				Namespace: "default",
			},
		}

		// Add different types of notifications
		collector.AddInfo("Info message", vs)
		collector.AddWarning("Warning message", vs)
		collector.AddError("Error message", nil)

		notifs := collector.GetNotifications()
		if len(notifs) != 3 {
			t.Errorf("Expected 3 notifications, got %d", len(notifs))
		}

		// Check notification types
		types := make(map[notifications.MessageType]int)
		for _, notif := range notifs {
			types[notif.Type]++
		}

		if types[notifications.InfoNotification] != 1 {
			t.Errorf("Expected 1 info notification, got %d", types[notifications.InfoNotification])
		}
		if types[notifications.WarningNotification] != 1 {
			t.Errorf("Expected 1 warning notification, got %d", types[notifications.WarningNotification])
		}
		if types[notifications.ErrorNotification] != 1 {
			t.Errorf("Expected 1 error notification, got %d", types[notifications.ErrorNotification])
		}
	})

	t.Run("formatted notifications", func(t *testing.T) {
		collector := NewNotificationCollector()

		collector.AddInfof("Processing %d VirtualServers", 5)
		collector.AddWarningf("Found %d unsupported fields in %s", 3, "test-vs")

		notifications := collector.GetNotifications()
		if len(notifications) != 2 {
			t.Errorf("Expected 2 notifications, got %d", len(notifications))
		}

		// Check message formatting
		if notifications[0].Message != "Processing 5 VirtualServers" {
			t.Errorf("Unexpected formatted message: %s", notifications[0].Message)
		}
		if notifications[1].Message != "Found 3 unsupported fields in test-vs" {
			t.Errorf("Unexpected formatted message: %s", notifications[1].Message)
		}
	})

	t.Run("collector operations", func(t *testing.T) {
		collector1 := NewNotificationCollector()
		collector2 := NewNotificationCollector()

		collector1.AddInfo("Message 1", nil)
		collector2.AddWarning("Message 2", nil)

		// Test merge
		collector1.Merge(collector2)
		if len(collector1.GetNotifications()) != 2 {
			t.Errorf("Expected 2 notifications after merge, got %d", len(collector1.GetNotifications()))
		}

		// Test clear
		collector1.Clear()
		if len(collector1.GetNotifications()) != 0 {
			t.Errorf("Expected 0 notifications after clear, got %d", len(collector1.GetNotifications()))
		}
	})
}
