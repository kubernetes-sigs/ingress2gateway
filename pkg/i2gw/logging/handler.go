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
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// ANSI color codes for terminal output.
const (
	colorReset        = "\033[0m"
	colorRed          = "\033[31m"
	colorYellow       = "\033[33m"
	colorBlue         = "\033[34m"
	colorGray         = "\033[90m"
	colorCyan         = "\033[36m"
	colorBrightGreen  = "\033[92m"
	colorBrightPurple = "\033[95m"
)

// PrettyHandler is an slog.Handler that outputs human-readable colored text.
type PrettyHandler struct {
	opts    *slog.HandlerOptions
	output  io.Writer
	mu      *sync.Mutex
	attrs   []slog.Attr
	groups  []string
	noColor bool
}

// NewPrettyHandler creates a new PrettyHandler that writes to the given output.
func NewPrettyHandler(output io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	return &PrettyHandler{
		opts:   opts,
		output: output,
		mu:     &sync.Mutex{},
	}
}

// DisableColor disables ANSI color codes in the output.
func (h *PrettyHandler) DisableColor() {
	h.noColor = true
}

// Enabled reports whether the handler handles records at the given level.
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}

	return level >= minLevel
}

// Handle handles the record by formatting it as colored text with box drawing.
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	levelStr, levelColor := formatLevel(r.Level)

	// Returns the color code, or empty string if color is disabled.
	c := func(code string) string {
		if h.noColor {
			return ""
		}
		return code
	}

	// Collect all attributes (from handler and record).
	allAttrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	allAttrs = append(allAttrs, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		allAttrs = append(allAttrs, a)
		return true
	})

	var lines []string

	// Top border with level.
	topBorder := fmt.Sprintf(
		"%s┌─ %s%s%s %s",
		c(colorGray),
		c(levelColor),
		levelStr,
		c(colorGray),
		strings.Repeat("─", 40),
	)
	lines = append(lines, topBorder+c(colorReset))

	// Message line.
	lines = append(lines, fmt.Sprintf("%s│%s  %s", c(colorGray), c(colorReset), r.Message))

	// Attribute lines.
	for _, attr := range allAttrs {
		if attr.Value.String() == "" {
			continue
		}
		var valueColor string
		switch attr.Key {
		case "provider":
			valueColor = colorBrightPurple
		case "object", "objects":
			valueColor = colorBrightGreen
		default:
			valueColor = ""
		}
		lines = append(lines,
			fmt.Sprintf(
				"%s│%s  %s%s:%s %s%s%s",
				c(colorGray), c(colorReset),
				c(colorGray), attr.Key, c(colorReset),
				c(valueColor), formatAttrValue(attr.Value), c(colorReset),
			),
		)
	}

	// Bottom border with source info.
	var bottomBorder string
	if h.opts.AddSource && r.PC != 0 {
		frame := sourceFrame(r.PC)
		bottomBorder = fmt.Sprintf(
			"%s└─ %s:%d%s",
			c(colorGray),
			filepath.Base(frame.File),
			frame.Line,
			c(colorReset),
		)
	} else {
		bottomBorder = fmt.Sprintf("%s└─%s", c(colorGray), c(colorReset))
	}
	lines = append(lines, bottomBorder)

	output := strings.Join(lines, "\n") + "\n"

	_, err := h.output.Write([]byte(output))

	return err
}

// WithAttrs returns a new handler with the given attributes.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &PrettyHandler{
		opts:    h.opts,
		output:  h.output,
		mu:      h.mu,
		attrs:   newAttrs,
		groups:  h.groups,
		noColor: h.noColor,
	}
}

// WithGroup returns a new handler with the given group.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &PrettyHandler{
		opts:    h.opts,
		output:  h.output,
		mu:      h.mu,
		attrs:   h.attrs,
		groups:  newGroups,
		noColor: h.noColor,
	}
}

// Returns the level string and its color.
func formatLevel(level slog.Level) (string, string) {
	// WARN and INFO have an extra space to ensure visual alignment with lines containing 5-letter
	// log levels.
	switch {
	case level >= slog.LevelError:
		return "ERROR", colorRed
	case level >= slog.LevelWarn:
		return "WARN ", colorYellow
	case level >= slog.LevelInfo:
		return "INFO ", colorCyan
	default:
		return "DEBUG", colorBlue
	}
}

// Formats an attribute value for display.
func formatAttrValue(v slog.Value) string {
	if v.Kind() == slog.KindString {
		s := v.String()
		// Quote strings which contain whitespace.
		if hasWhitespace(s) {
			return fmt.Sprintf("%q", s)
		}
		return s
	}

	return v.String()
}

// Checks if a string contains any whitespace.
func hasWhitespace(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			return true
		}
	}

	return false
}

// Returns the runtime frame for the given program counter.
func sourceFrame(pc uintptr) runtime.Frame {
	frames := runtime.CallersFrames([]uintptr{pc})
	frame, _ := frames.Next()

	return frame
}
