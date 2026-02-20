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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrettyHandlerEnabled(t *testing.T) {
	tests := []struct {
		desc         string
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
		t.Run(tc.desc, func(t *testing.T) {
			h := NewPrettyHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: tc.handlerLevel})
			got := h.Enabled(context.Background(), tc.recordLevel)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestPrettyHandlerHandle(t *testing.T) {
	tests := []struct {
		desc     string
		level    slog.Level
		message  string
		attrs    []slog.Attr
		contains []string
	}{
		{
			desc:     "info message",
			level:    slog.LevelInfo,
			message:  "foo bar",
			attrs:    nil,
			contains: []string{"INFO", "foo bar"},
		},
		{
			desc:     "warning with attrs",
			level:    slog.LevelWarn,
			message:  "something happened",
			attrs:    []slog.Attr{slog.String("key", "value")},
			contains: []string{"WARN", "something happened", "key:", "value"},
		},
		{
			desc:     "error message",
			level:    slog.LevelError,
			message:  "bad thing",
			attrs:    nil,
			contains: []string{"ERROR", "bad thing"},
		},
		{
			desc:     "debug message",
			level:    slog.LevelDebug,
			message:  "debug info",
			attrs:    nil,
			contains: []string{"DEBUG", "debug info"},
		},
		{
			desc:    "quoted value with space",
			level:   slog.LevelInfo,
			message: "test",
			attrs:   []slog.Attr{slog.String("key", "value with space")},
			// The value should be quoted.
			contains: []string{"key:", "\"value with space\""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewPrettyHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

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
				require.Contains(t, output, s)
			}
		})
	}
}

func TestPrettyHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, nil)

	h2 := h.WithAttrs([]slog.Attr{slog.String("base", "attr")})

	logger := slog.New(h2)
	logger.Info("test message", "extra", "value")

	output := buf.String()
	require.Contains(t, output, "base:")
	require.Contains(t, output, "attr")
	require.Contains(t, output, "extra:")
	require.Contains(t, output, "value")
}

func TestPrettyHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, nil)

	h2 := h.WithGroup("mygroup")

	require.NotNil(t, h2)

	logger := slog.New(h2)
	logger.Info("grouped message")

	output := buf.String()
	require.Contains(t, output, "grouped message")
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
			require.Equal(t, tc.expectedStr, str)
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
			got := hasWhitespace(tc.input)
			require.Equal(t, tc.expected, got)
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
	require.Contains(t, output, "handler_test.go:")
}

func TestPrettyHandlerNoColor(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h.DisableColor()

	logger := slog.New(h)
	logger.Info("no color message", "key", "value")

	output := buf.String()
	require.Contains(t, output, "INFO")
	require.Contains(t, output, "no color message")
	require.Contains(t, output, "key:")
	require.Contains(t, output, "value")
	require.NotContains(t, output, "\033[", "output should not contain ANSI escape codes")
}

func TestPrettyHandlerNoColorWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	h.DisableColor()

	h2 := h.WithAttrs([]slog.Attr{slog.String("provider", "nginx")})

	logger := slog.New(h2)
	logger.Info("test")

	output := buf.String()
	require.NotContains(t, output, "\033[", "noColor should propagate through WithAttrs")
}
