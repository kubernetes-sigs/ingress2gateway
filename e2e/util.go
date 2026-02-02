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

package e2e

import (
	"crypto/rand"

	"sigs.k8s.io/yaml"
)

// Generates a cryptographically random alphanumeric string of length n. Uses crypto/rand to ensure
// uniqueness even when called from parallel tests.
func randString(n int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	// Populate b with random alphanumeric characters by indexing chars with the remainder of
	// (b[i] / len(chars)). This ensures we always select a valid element from chars. The byte()
	// conversion is required because the % operator expects two operands of the same type.
	for i := range b {
		b[i] = chars[b[i]%byte(len(chars))]
	}

	return string(b), nil
}

// Converts a k8s object to a YAML string.
func toYAML(obj interface{}) (string, error) {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
