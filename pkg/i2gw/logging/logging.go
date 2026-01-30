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
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Format represents the output format for logs.
type Format string

const (
	// FormatText outputs human-readable colored text.
	FormatText Format = "text"
	// FormatJSON outputs machine-parsable JSON.
	FormatJSON Format = "json"
)

// Config holds the configuration for the logger.
type Config struct {
	// Format specifies the output format (text or json).
	Format Format
	// Level specifies the minimum log level to output.
	Level slog.Level
	// Output specifies where logs are written (defaults to os.Stderr).
	Output io.Writer
	// NoColor disables ANSI color codes in text format output.
	NoColor bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Format: FormatText,
		Level:  slog.LevelInfo,
		Output: os.Stderr,
	}
}

// Init initializes the global slog logger with the provided configuration. Should be called once
// at program startup.
func Init(cfg Config) {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}

	var handler slog.Handler

	switch cfg.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(cfg.Output, &slog.HandlerOptions{
			AddSource: true,
			Level:     cfg.Level,
		})
	case FormatText:
		pretty := NewPrettyHandler(cfg.Output, &slog.HandlerOptions{
			AddSource: true,
			Level:     cfg.Level,
		})
		if cfg.NoColor {
			pretty.DisableColor()
		}
		handler = pretty
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// ParseLevel converts a string level name to an slog.Level. Supported values: debug, info, warn,
// error (case-insensitive). Returns slog.LevelInfo for unknown values.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ParseFormat converts a string format name to a Format. Supported values: text, json
// (case-insensitive). Returns FormatText for unknown values.
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "text":
		return FormatText
	default:
		return FormatText
	}
}

// WithProvider returns a logger with the provider attribute pre-set.
func WithProvider(providerName string) *slog.Logger {
	return slog.Default().With(slog.String("provider", providerName))
}

// WithEmitter returns a logger with the emitter attribute pre-set.
func WithEmitter(emitterName string) *slog.Logger {
	return slog.Default().With(slog.String("emitter", emitterName))
}

// ObjectRef returns an slog attribute for a Kubernetes object reference. The format is
// "Kind:namespace/name" or "Kind:name" for cluster-scoped objects.
func ObjectRef(obj client.Object) slog.Attr {
	if obj == nil {
		return slog.String("object", "")
	}

	return slog.String("object", objToString(obj))
}

// ObjectRefs returns an slog attribute for multiple Kubernetes object references.
func ObjectRefs(objs ...client.Object) slog.Attr {
	if len(objs) == 0 {
		return slog.String("objects", "")
	}

	refs := make([]string, 0, len(objs))
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		refs = append(refs, objToString(obj))
	}

	return slog.String("objects", strings.Join(refs, ", "))
}

// Noop returns a no-op logger for use in tests.
func Noop() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// Returns a string representing the object.
func objToString(obj client.Object) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	key := client.ObjectKeyFromObject(obj)

	if key.Namespace != "" {
		return fmt.Sprintf("%s:%s/%s", kind, key.Namespace, key.Name)
	}

	return fmt.Sprintf("%s:%s", kind, key.Name)
}
