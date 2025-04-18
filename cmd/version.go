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
	"runtime/debug"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of ingress2gateway",
	Long:  "Prints the build version details for ingress2gateway, including Git status information and Go version",
	Run: func(_ *cobra.Command, _ []string) {
		printVersion() // Call the helper function
	},
}

// printVersion formats and prints the version information.
func printVersion() {
	fmt.Printf("ingress2gateway version: %s\n", i2gw.Version)

	// Print the golang version if it's available
	buildInfo, ok := debug.ReadBuildInfo()
	if ok && buildInfo != nil {
		fmt.Printf("Built with Go version: %s\n", buildInfo.GoVersion)
	}
}
