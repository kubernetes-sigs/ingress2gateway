/*
Copyright 2025 The Kubernetes Authors.

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

package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newGenerateDocsCmd creates a command for generating documentation files.
// This command is hidden from the regular help output because it should only be used by the hack/generate-cli-doc.sh script.
// It generates Markdown documentation for all commands and subcommands of the provided root command.
func newGenerateDocsCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-docs",
		Short:  "Generate documentation files",
		Hidden: true, // Hide from regular help output
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, c := range rootCmd.Commands() {
				if c.Hidden {
					continue // Skip hidden commands
				}
				if c.Name() == "help" {
					continue // Skip help command
				}
				err := generateRecursiveCommandDocs(c, cmd.OutOrStdout())
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}

// generateRecursiveCommandDocs generates Markdown for a command and its subcommands.
func generateRecursiveCommandDocs(cmd *cobra.Command, w io.Writer) error {
	// Generate docs for the current command first
	if err := generateCommandDocs(cmd, w); err != nil {
		return err
	}

	// Recursively generate for subcommands
	subCmds := cmd.Commands()
	sort.Slice(subCmds, func(i, j int) bool { // Sort for consistent order
		return subCmds[i].Name() < subCmds[j].Name()
	})
	for _, subCmd := range subCmds {
		fmt.Fprintln(w, "\n---") // Add separator
		if err := generateRecursiveCommandDocs(subCmd, w); err != nil {
			return err
		}
	}
	return nil
}

func generateCommandDocs(cmd *cobra.Command, w io.Writer) error {
	cmd.InitDefaultHelpCmd()     // Ensure help command is initialized
	cmd.InitDefaultVersionFlag() // Ensure version flag is initialized

	// Print Header
	fmt.Fprintf(w, "### `%s`\n\n", cmd.CommandPath())

	// Handle Usage
	// Show usage if the command itself is runnable OR if it has subcommands
	if cmd.Runnable() || cmd.HasSubCommands() {
		fmt.Fprintf(w, "**Usage:** `%s`\n\n", cmd.UseLine())
	}

	// Handle Aliases
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(w, "**Aliases:** `%s`\n\n", strings.Join(cmd.Aliases, "`, `"))
	}

	// Handle Description
	description := cmd.Long
	if description == "" {
		description = cmd.Short
	}
	if description != "" {
		fmt.Fprintf(w, "**Description:**\n\n%s\n\n", description)
	}

	// Handle Subcommands
	if cmd.HasSubCommands() {
		fmt.Fprintln(w, "**Available Subcommands:**")
		fmt.Fprintln(w, "| Command | Description |")
		fmt.Fprintln(w, "| ------- | ----------- |")

		subCmds := cmd.Commands()
		sort.Slice(subCmds, func(i, j int) bool { // Sort for consistent output
			return subCmds[i].Name() < subCmds[j].Name()
		})

		for _, subCmd := range subCmds {
			// Only list available (non-hidden, non-help) commands
			if subCmd.IsAvailableCommand() {
				fmt.Fprintf(w, "| `%s` | %s |\n", subCmd.Name(), subCmd.Short)
			}
		}
		// Newline
		fmt.Fprintln(w, "")
	}

	// Handle Flags
	// Check if *any* flags (local or inherited) are defined for the command
	if cmd.HasAvailableFlags() {
		fmt.Fprintln(w, "**Flags:**")
		if err := generateFlagTable(cmd, w); err != nil {
			return fmt.Errorf("error generating flag table for %s: %w", cmd.Name(), err)
		}
		fmt.Fprintln(w, "")
	} else if !cmd.HasSubCommands() {
		// Only print "Flags: None" if it's a leaf command with no flags *and* no subcommands
		fmt.Fprintln(w, "**Flags:** None")
	}

	// Handle Examples
	if cmd.Example != "" {
		fmt.Fprintf(w, "**Example:**\n\n```\n%s\n```\n\n", cmd.Example)
	}
	return nil
}

// generateFlagTable generates a Markdown table for a command's flags.
// It is a wrapper around generateFlagTable to explicitly handle both local and inherited flags.
func generateFlagTable(cmd *cobra.Command, w io.Writer) error {
	if err := generateFlagTableHelper(cmd.LocalFlags(), cmd.Name(), false /*inherited*/, w); err != nil {
		return err
	}

	if err := generateFlagTableHelper(cmd.InheritedFlags(), cmd.Name(), true /*inherited*/, w); err != nil {
		return err
	}
	return nil
}

// generateFlagTableHelper generates a Markdown table for a command's flags,
// cmdName is passed in explicitly since it cannot be obtained from the flag set.
// inherited is used to generate a separate header for inherited flags.
func generateFlagTableHelper(flagSet *pflag.FlagSet, cmdName string, inherited bool, w io.Writer) error {
	// Check if there are any flags to print.
	flagSlice := make([]*pflag.Flag, 0)
	flagSet.VisitAll(func(flag *pflag.Flag) {
		// Filter out the standard help flag unless documenting the help command itself
		if flag.Name == "help" && flag.Usage == "help for "+cmdName {
			if cmdName != "help" {
				return // Skip standard help flag
			}
		}
		// Filter out test flags if necessary (e.g., if running doc gen via `go test`)
		if strings.HasPrefix(flag.Name, "test.") {
			return
		}
		flagSlice = append(flagSlice, flag)
	})

	if len(flagSlice) == 0 {
		return nil // No printable flags found
	}

	// Sort flags by name for consistent output.
	sort.Slice(flagSlice, func(i, j int) bool {
		return flagSlice[i].Name < flagSlice[j].Name
	})

	// Print inherited flags in a separate section.
	if inherited {
		fmt.Fprintln(w, "\n**Inherited Flags:**")
	}

	// Print Header.
	fmt.Fprintln(w, "| Flag | Default Value | Required | Description |")
	fmt.Fprintln(w, "| ---- | ------------- | -------- | ----------- |")

	// Print table rows.
	for _, flag := range flagSlice {
		required := "No"
		// This looks weird but this actually how Cobra marks required flags.
		if _, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]; ok {
			required = "Yes"
		}

		defaultValue := flag.DefValue
		if defaultValue == "" {
			// Represent empty default clearly in Markdown
			defaultValue = "` ` "
		} else {
			// Escape potential pipes and wrap default value in backticks
			defaultValue = fmt.Sprintf("`%s`", strings.ReplaceAll(defaultValue, "|", "\\|"))
		}

		// Escape pipes in description
		description := strings.ReplaceAll(flag.Usage, "|", "\\|")

		// Format flag name (including shorthand if available and not deprecated)
		flagName := fmt.Sprintf("`--%s`", flag.Name)
		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			flagName = fmt.Sprintf("`-%s`, %s", flag.Shorthand, flagName)
		}

		fmt.Fprintf(w, "| %s | %s | %s | %s |\n",
			flagName,
			defaultValue,
			required,
			description,
		)
	}
	return nil
}
