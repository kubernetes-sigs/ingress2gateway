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

package cmd

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var generateReadmeFlags = map[string]struct{}{
	"print": {},
}

func newListFlagsCommand(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "list-flags",
		Short: "Lists all flags for documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			table := tablewriter.NewWriter(os.Stdout)
			printCommandFlags(rootCmd, table)
			return nil
		},
	}
}

func printCommandFlags(cmd *cobra.Command, table *tablewriter.Table) {
	if _, ok := generateReadmeFlags[cmd.Use]; ok {
		fmt.Printf("\n### `%s` command\n\n", cmd.Use)

		table.SetHeader([]string{"Flag", "Default Value", "Required", "Description"})

		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			table.Append([]string{
				flag.Name,
				flag.DefValue,
				fmt.Sprintf("%v", flag.NoOptDefVal == ""),
				flag.Usage,
			})
		})

		table.Render()

		table.ClearRows()
	}

	for _, subCmd := range cmd.Commands() {
		printCommandFlags(subCmd, table)
	}
}
