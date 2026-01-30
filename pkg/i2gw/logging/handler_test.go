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
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestPrettyHandlerEnabled(t *testing.T) {
	tests := []struct {
		name         string
		handlerLevel slog.Level
		recordLevel  slog.Level
		expected     bool
	}{
		{"info enabled at info", slog.LevelInfo, slog.LevelInfo, true},
		{"warn enabled at info", slog.LevelInfo, slog.LevelWarn, true},
		{"debug disabled at info", slog.LevelInfo, slog.LevelDebug, false},
		{"info disabled at warn", slog.LevelWarn, slog.LevelInfo, false},
		{"error enabled at warn", slog.LevelWarn, slog.LevelError, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewPrettyHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: tc.handlerLevel})
			got := h.Enabled(context.Background(), tc.recordLevel)
			if got != tc.expected {
				t.Errorf("Enabled(%v) = %v, want %v", tc.recordLevel, got, tc.expected)
			}
		})
	}
}

func TestPrettyHandlerHandle(t *testing.T) {
	tests := []struct {
		name     string
		level    slog.Level
		message  string
		attrs    []slog.Attr
		contains []string
	}{
		{
			name:     "info message",
			level:    slog.LevelInfo,
			message:  "hello world",
			attrs:    nil,
			contains: []string{"INFO", "hello world"},
		},
		{
			name:     "warning with attrs",
			level:    slog.LevelWarn,
			message:  "something happened",
			attrs:    []slog.Attr{slog.String("key", "value")},
			contains: []string{"WARN", "something happened", "key:", "value"},
		},
		{
			name:     "error message",
			level:    slog.LevelError,
			message:  "bad thing",
			attrs:    nil,
			contains: []string{"ERROR", "bad thing"},
		},
		{
			name:     "debug message",
			level:    slog.LevelDebug,
			message:  "debug info",
			attrs:    nil,
			contains: []string{"DEBUG", "debug info"},
		},
		{
			name:    "quoted value with space",
			level:   slog.LevelInfo,
			message: "test",
			attrs:   []slog.Attr{slog.String("key", "value with space")},
			// The value should be quoted.
			contains: []string{"key:", "\"value with space\""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewPrettyHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

			// Create a proper logger and log with it.
			logger := slog.New(h)
			args := make([]any, 0, len(tc.attrs)*2)
			for _, attr := range tc.attrs {
				args = append(args, attr.Key, attr.Value.Any())
			}

			switch tc.level {
			case slog.LevelDebug:
				logger.Debug(tc.message, args...)
			case slog.LevelInfo:
				logger.Info(tc.message, args...)
			case slog.LevelWarn:
				logger.Warn(tc.message, args...)
			case slog.LevelError:
				logger.Error(tc.message, args...)
			}

			output := buf.String()
			for _, s := range tc.contains {
				if !strings.Contains(output, s) {
					t.Errorf("expected output to contain %q, got %q", s, output)
				}
			}
		})
	}
}

func TestPrettyHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, nil)

	// Add some attributes.
	h2 := h.WithAttrs([]slog.Attr{slog.String("base", "attr")})

	logger := slog.New(h2)
	logger.Info("test message", "extra", "value")

	output := buf.String()
	if !strings.Contains(output, "base:") || !strings.Contains(output, "attr") {
		t.Errorf("expected output to contain 'base:' and 'attr', got %q", output)
	}
	if !strings.Contains(output, "extra:") || !strings.Contains(output, "value") {
		t.Errorf("expected output to contain 'extra:' and 'value', got %q", output)
	}
}

func TestPrettyHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, nil)

	h2 := h.WithGroup("mygroup")

	// Verify it returns a new handler (basic check).
	if h2 == nil {
		t.Error("WithGroup returned nil")
	}

	// The handler should still work.
	logger := slog.New(h2)
	logger.Info("grouped message")

	output := buf.String()
	if !strings.Contains(output, "grouped message") {
		t.Errorf("expected output to contain 'grouped message', got %q", output)
	}
}

func TestFormatLevel(t *testing.T) {
	tests := []struct {
		level       slog.Level
		expectedStr string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO "},
		{slog.LevelWarn, "WARN "},
		{slog.LevelError, "ERROR"},
	}

	for _, tc := range tests {
		t.Run(tc.expectedStr, func(t *testing.T) {
			str, _ := formatLevel(tc.level)
			if str != tc.expectedStr {
				t.Errorf("formatLevel(%v) = %q, want %q", tc.level, str, tc.expectedStr)
			}
		})
	}
}

func TestContainsSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello", false},
		{"hello world", true},
		{"hello\tworld", true},
		{"hello\nworld", true},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := containsSpace(tc.input)
			if got != tc.expected {
				t.Errorf("containsSpace(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestPrettyHandlerAddSource(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	})

	logger := slog.New(h)
	logger.Info("test with source")

	output := buf.String()
	// Should contain the filename and line number.
	if !strings.Contains(output, "handler_test.go:") {
		t.Errorf("expected output to contain source file info, got %q", output)
	}
}
