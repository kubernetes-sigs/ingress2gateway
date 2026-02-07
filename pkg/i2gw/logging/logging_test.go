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

package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseLevel(tc.input)
			if got != tc.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"text", FormatText},
		{"TEXT", FormatText},
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"unknown", FormatText},
		{"", FormatText},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseFormat(tc.input)
			if got != tc.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestWithProvider(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Format: FormatJSON,
		Level:  slog.LevelInfo,
		Output: &buf,
	}
	Init(cfg)

	logger := WithProvider("nginx")
	logger.Info("test message")

	output := buf.String()

	// Verify it's valid JSON with the provider attribute.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v, output: %q", err, output)
	}

	if result["provider"] != "nginx" {
		t.Errorf("expected provider='nginx', got %v", result["provider"])
	}

	if result["msg"] != "test message" {
		t.Errorf("expected msg='test message', got %v", result["msg"])
	}
}

func TestObjectRef(t *testing.T) {
	tests := []struct {
		name     string
		obj      func() *gatewayv1.Gateway
		expected string
	}{
		{
			name: "namespaced object",
			obj: func() *gatewayv1.Gateway {
				gw := &gatewayv1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-gateway",
						Namespace: "default",
					},
				}
				gw.SetGroupVersionKind(gatewayv1.SchemeGroupVersion.WithKind("Gateway"))
				return gw
			},
			expected: "Gateway:default/my-gateway",
		},
		{
			name: "cluster-scoped object (empty namespace)",
			obj: func() *gatewayv1.Gateway {
				gw := &gatewayv1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-gateway",
					},
				}
				gw.SetGroupVersionKind(gatewayv1.SchemeGroupVersion.WithKind("Gateway"))
				return gw
			},
			expected: "Gateway:my-gateway",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attr := ObjectRef(tc.obj())
			if attr.Key != "object" {
				t.Errorf("ObjectRef attr key = %q, want %q", attr.Key, "object")
			}
			if attr.Value.String() != tc.expected {
				t.Errorf("ObjectRef attr value = %q, want %q", attr.Value.String(), tc.expected)
			}
		})
	}
}

func TestObjectRefNil(t *testing.T) {
	attr := ObjectRef(nil)
	if attr.Key != "object" {
		t.Errorf("ObjectRef(nil) attr key = %q, want %q", attr.Key, "object")
	}
	if attr.Value.String() != "" {
		t.Errorf("ObjectRef(nil) attr value = %q, want empty string", attr.Value.String())
	}
}

func TestObjectRefs(t *testing.T) {
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-gateway",
			Namespace: "default",
		},
	}
	gw.SetGroupVersionKind(gatewayv1.SchemeGroupVersion.WithKind("Gateway"))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "prod",
		},
	}
	svc.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))

	attr := ObjectRefs(gw, svc)
	if attr.Key != "objects" {
		t.Errorf("ObjectRefs attr key = %q, want %q", attr.Key, "objects")
	}

	expected := "Gateway:default/my-gateway, Service:prod/my-service"
	if attr.Value.String() != expected {
		t.Errorf("ObjectRefs attr value = %q, want %q", attr.Value.String(), expected)
	}
}

func TestObjectRefsEmpty(t *testing.T) {
	attr := ObjectRefs()
	if attr.Key != "objects" {
		t.Errorf("ObjectRefs() attr key = %q, want %q", attr.Key, "objects")
	}
	if attr.Value.String() != "" {
		t.Errorf("ObjectRefs() attr value = %q, want empty string", attr.Value.String())
	}
}

func TestSetupTextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Format: FormatText,
		Level:  slog.LevelInfo,
		Output: &buf,
	}
	Init(cfg)

	slog.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected output to contain INFO, got %q", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %q", output)
	}
	if !strings.Contains(output, "key:") || !strings.Contains(output, "value") {
		t.Errorf("expected output to contain 'key:' and 'value', got %q", output)
	}
}

func TestSetupJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Format: FormatJSON,
		Level:  slog.LevelInfo,
		Output: &buf,
	}
	Init(cfg)

	slog.Info("test message", "key", "value")

	output := buf.String()

	// Verify it's valid JSON.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v, output: %q", err, output)
	}

	// Verify expected fields.
	if result["msg"] != "test message" {
		t.Errorf("expected msg='test message', got %v", result["msg"])
	}
	if result["key"] != "value" {
		t.Errorf("expected key='value', got %v", result["key"])
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Format: FormatText,
		Level:  slog.LevelWarn,
		Output: &buf,
	}
	Init(cfg)

	slog.Info("info message")
	slog.Warn("warn message")

	output := buf.String()
	if strings.Contains(output, "info message") {
		t.Errorf("expected info message to be filtered out, got %q", output)
	}
	if !strings.Contains(output, "warn message") {
		t.Errorf("expected warn message to be present, got %q", output)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Format != FormatText {
		t.Errorf("DefaultConfig().Format = %v, want %v", cfg.Format, FormatText)
	}
	if cfg.Level != slog.LevelInfo {
		t.Errorf("DefaultConfig().Level = %v, want %v", cfg.Level, slog.LevelInfo)
	}
}
