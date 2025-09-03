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

package crds

import (
	"testing"

	nginxv1 "github.com/nginx/kubernetes-ingress/pkg/apis/configuration/v1"
)

func TestContainsRegexPatterns(t *testing.T) {
	t.Run("regex pattern detection", func(t *testing.T) {
		tests := []struct {
			input    string
			expected bool
		}{
			{"simple-path", false},
			{"/api/v1", false},
			{"/api/.*", true},
			{"/app/[0-9]+", true},
			{"/test?param=value", true},
			{"regular-string", false},
			{"/path/with/dots.extension", true},
			{"/path^start", true},
			{"/path$end", true},
			{"/path(group)", true},
			{"/path{1,3}", true},
			{"/path|alternative", true},
		}

		for _, test := range tests {
			result := containsRegexPatterns(test.input)
			if result != test.expected {
				t.Errorf("containsRegexPatterns(%q) = %v, expected %v", test.input, result, test.expected)
			}
		}
	})
}

func TestFindUpstream(t *testing.T) {
	upstreams := []nginxv1.Upstream{
		{Name: "upstream1", Service: "service1", Port: 80},
		{Name: "upstream2", Service: "service2", Port: 8080},
		{Name: "upstream3", Service: "service3", Port: 3000},
	}

	t.Run("find existing upstream", func(t *testing.T) {
		result := findUpstream(upstreams, "upstream2")
		if result == nil {
			t.Error("Expected to find upstream2, got nil")
		}
		if result.Service != "service2" {
			t.Errorf("Expected service2, got %s", result.Service)
		}
		if result.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", result.Port)
		}
	})

	t.Run("upstream not found", func(t *testing.T) {
		result := findUpstream(upstreams, "nonexistent")
		if result != nil {
			t.Error("Expected nil for nonexistent upstream, got result")
		}
	})
}