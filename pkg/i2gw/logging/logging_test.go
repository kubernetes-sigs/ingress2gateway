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
	"testing"

	"github.com/stretchr/testify/require"
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
			require.Equal(t, tc.expected.String(), ParseLevel(tc.input).String())
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
			require.Equal(t, tc.expected, ParseFormat(tc.input))
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

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &result), "output: %s", output)

	require.Equal(t, "nginx", result["provider"])
	require.Equal(t, "test message", result["msg"])
}

func TestWithEmitter(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Format: FormatJSON,
		Level:  slog.LevelInfo,
		Output: &buf,
	}
	Init(cfg)

	logger := WithEmitter("envoy-gateway")
	logger.Info("test message")

	output := buf.String()

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &result), "output: %s", output)

	require.Equal(t, "envoy-gateway", result["emitter"])
	require.Equal(t, "test message", result["msg"])
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
			require.Equal(t, "object", attr.Key)
			require.Equal(t, tc.expected, attr.Value.String())
		})
	}
}

func TestObjectRefNil(t *testing.T) {
	attr := ObjectRef(nil)
	require.Equal(t, "object", attr.Key)
	require.Empty(t, attr.Value.String())
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
	require.Equal(t, "objects", attr.Key)
	require.Equal(t, "Gateway:default/my-gateway, Service:prod/my-service", attr.Value.String())
}

func TestObjectRefsEmpty(t *testing.T) {
	attr := ObjectRefs()
	require.Equal(t, "objects", attr.Key)
	require.Empty(t, attr.Value.String())
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
	require.Contains(t, output, "INFO")
	require.Contains(t, output, "test message")
	require.Contains(t, output, "key:")
	require.Contains(t, output, "value")
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

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &result), "output: %s", output)

	require.Equal(t, "test message", result["msg"])
	require.Equal(t, "value", result["key"])
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
	require.NotContains(t, output, "info message")
	require.Contains(t, output, "warn message")
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.Equal(t, FormatText, cfg.Format)
	require.Equal(t, slog.LevelInfo, cfg.Level)
}
