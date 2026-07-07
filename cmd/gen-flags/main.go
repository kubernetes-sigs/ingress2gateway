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

// Command gen-flags regenerates the CLI flags tables in README.md from the
// cobra command definitions, so the documentation never drifts from the real
// flags. It is invoked by `make gen-flags` and verified by `make verify-flags`
// (hack/verify-flags.sh).
//
// The generated content is written between the marker comments:
//
//	<!-- BEGIN GENERATED FLAGS -->
//	<!-- END GENERATED FLAGS -->
//
// Run it with the path to README.md as the only argument:
//
//	go run ./cmd/gen-flags README.md
//
// With --check it does not modify the file; instead it exits non-zero if the
// file is out of date.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kubernetes-sigs/ingress2gateway/cmd"
)

const (
	markerStart = "<!-- BEGIN GENERATED FLAGS -->"
	markerEnd   = "<!-- END GENERATED FLAGS -->"

	// providerFlagUsagePrefix is the prefix the print command prepends to the
	// usage string of every provider-specific flag (see cmd/print.go). It is
	// used here to separate provider-specific flags from the general flags.
	providerFlagUsagePrefix = "Provider-specific:"
)

func main() {
	check := flag.Bool("check", false, "verify README is up to date instead of writing it; exit non-zero if it is stale")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: gen-flags [--check] <path-to-README.md>")
		os.Exit(2)
	}
	readmePath := flag.Arg(0)

	if err := run(readmePath, *check); err != nil {
		fmt.Fprintf(os.Stderr, "gen-flags: %v\n", err)
		os.Exit(1)
	}
}

func run(readmePath string, check bool) error {
	original, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", readmePath, err)
	}

	generated := renderFlagsSection()

	updated, err := replaceBetweenMarkers(string(original), generated)
	if err != nil {
		return err
	}

	if check {
		if updated != string(original) {
			return fmt.Errorf("%s is out of date; run `make gen-flags` and commit the result", readmePath)
		}
		return nil
	}

	if updated == string(original) {
		return nil
	}
	if err := os.WriteFile(readmePath, []byte(updated), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", readmePath, err)
	}
	return nil
}

// flagDoc is a flattened, render-ready view of a single pflag.Flag.
type flagDoc struct {
	name      string
	shorthand string
	defValue  string
	required  bool
	usage     string
	provider  bool
}

// collectFlags walks the whole command tree and returns the documented flags,
// de-duplicated by name. Persistent flags defined on the root command (e.g.
// --kubeconfig, --no-color) are inherited by every subcommand, so visiting all
// commands captures both root-level and `print`-level flags.
func collectFlags() []flagDoc {
	root := cmd.NewRootCommand()

	seen := map[string]flagDoc{}
	var visit func(c *cobra.Command)
	visit = func(c *cobra.Command) {
		collect := func(fs *pflag.FlagSet) {
			fs.VisitAll(func(f *pflag.Flag) {
				if f.Hidden {
					return
				}
				// Skip the auto-generated help flag; it is not part of the
				// documented surface.
				if f.Name == "help" {
					return
				}
				if _, ok := seen[f.Name]; ok {
					return
				}
				required := false
				if vals, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok {
					for _, v := range vals {
						if v == "true" {
							required = true
						}
					}
				}
				seen[f.Name] = flagDoc{
					name:      f.Name,
					shorthand: f.Shorthand,
					defValue:  f.DefValue,
					required:  required,
					usage:     f.Usage,
					provider:  strings.HasPrefix(f.Usage, providerFlagUsagePrefix),
				}
			})
		}
		collect(c.PersistentFlags())
		collect(c.LocalFlags())
		for _, sub := range c.Commands() {
			visit(sub)
		}
	}
	visit(root)

	docs := make([]flagDoc, 0, len(seen))
	for _, d := range seen {
		docs = append(docs, d)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].name < docs[j].name })
	return docs
}

func renderFlagsSection() string {
	docs := collectFlags()

	var general, provider []flagDoc
	for _, d := range docs {
		if d.provider {
			provider = append(provider, d)
		} else {
			general = append(general, d)
		}
	}

	var b strings.Builder
	b.WriteString("### `print` command\n\n")
	b.WriteString("| Flag | Short | Default Value | Required | Description |\n")
	b.WriteString("| ---- | ----- | ------------- | -------- | ----------- |\n")
	for _, d := range general {
		fmt.Fprintf(&b, "| `--%s` | %s | %s | %s | %s |\n",
			d.name,
			shortCell(d.shorthand),
			defaultCell(d.defValue),
			requiredCell(d.required),
			usageCell(d.usage),
		)
	}

	if len(provider) > 0 {
		b.WriteString("\n#### Provider-specific flags\n\n")
		b.WriteString("| Flag | Default Value | Required | Description |\n")
		b.WriteString("| ---- | ------------- | -------- | ----------- |\n")
		for _, d := range provider {
			fmt.Fprintf(&b, "| `--%s` | %s | %s | %s |\n",
				d.name,
				defaultCell(d.defValue),
				requiredCell(d.required),
				usageCell(d.usage),
			)
		}
	}

	return b.String()
}

func shortCell(shorthand string) string {
	if shorthand == "" {
		return ""
	}
	return "`-" + shorthand + "`"
}

func defaultCell(def string) string {
	if def == "" {
		return ""
	}
	return "`" + escapeCell(def) + "`"
}

func requiredCell(required bool) string {
	if required {
		return "Yes"
	}
	return "No"
}

// usageCell flattens a flag usage string into a single Markdown table cell.
func usageCell(usage string) string {
	// Some usage strings span multiple lines (e.g. --all-namespaces); collapse
	// them so the table row stays valid.
	usage = strings.ReplaceAll(usage, "\n", " ")
	usage = strings.Join(strings.Fields(usage), " ")
	return escapeCell(usage)
}

func escapeCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func replaceBetweenMarkers(content, generated string) (string, error) {
	startIdx := strings.Index(content, markerStart)
	if startIdx < 0 {
		return "", fmt.Errorf("start marker %q not found in README", markerStart)
	}
	endIdx := strings.Index(content, markerEnd)
	if endIdx < 0 {
		return "", fmt.Errorf("end marker %q not found in README", markerEnd)
	}
	if endIdx < startIdx {
		return "", fmt.Errorf("end marker appears before start marker in README")
	}

	var b bytes.Buffer
	// Keep everything up to and including the start marker line.
	b.WriteString(content[:startIdx+len(markerStart)])
	b.WriteString("\n")
	b.WriteString(generated)
	b.WriteString(content[endIdx:])
	return b.String(), nil
}
