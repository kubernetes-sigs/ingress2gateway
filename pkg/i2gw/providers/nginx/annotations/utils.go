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

package annotations

import (
	"fmt"
	"regexp"
	"strings"
)

// splitAndTrimCommaList splits a comma-separated string and trims whitespace from each part
func splitAndTrimCommaList(input string) []string {
	if input == "" {
		return nil
	}

	var result []string
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// Regular expression that captures the number and unit for NGINX size (example: used for client body size)
var nginxSizeRegex = regexp.MustCompile(`^(?i)(\d+)([bkmg]?)$`)

// convertNginxSizeToK8sQuantity converts nginx size format to Kubernetes resource.Quantity format.
// NGINX uses binary units (e.g. k = 2^10) while in Kubernetes, k = 1000 and Ki = 2^10.
func convertNginxSizeToK8sQuantity(nginxSize string) (string, error) {
	nginxSize = strings.TrimSpace(nginxSize)

	matches := nginxSizeRegex.FindStringSubmatch(nginxSize)
	if matches == nil {
		return "", fmt.Errorf("invalid nginx size format: %q", nginxSize)
	}

	number := matches[1]
	unit := matches[2]

	// Convert nginx unit to K8s Quantity unit
	switch strings.ToLower(unit) {
	case "b", "":
		return number, nil
	case "k":
		return number + "Ki", nil
	case "m":
		return number + "Mi", nil
	case "g":
		return number + "Gi", nil
	default:
		return "", fmt.Errorf("unsupported nginx size unit: %q", unit)
	}
}
