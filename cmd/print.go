/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"strings"
)

var (
	output = "yaml"
)

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		resourcePrinter := getResourcePrinter()
		i2gw.Run(resourcePrinter)
	},
}

func getResourcePrinter() printers.ResourcePrinter {
	if output == "json" {
		return &printers.JSONPrinter{}
	}
	return &printers.YAMLPrinter{}
}

func init() {
	var printFlags genericclioptions.JSONYamlPrintFlags
	allowedFormats := printFlags.AllowedFormats()

	printCmd.Flags().StringVarP(&output, "output", "o", "yaml",
		fmt.Sprintf(`Output format. One of: (%s)`, strings.Join(allowedFormats, ", ")))

	rootCmd.AddCommand(printCmd)
}
