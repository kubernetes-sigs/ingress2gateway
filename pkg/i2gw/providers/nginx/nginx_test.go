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

package nginx

import (
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name string
		conf *i2gw.ProviderConf
	}{
		{
			name: "basic",
			conf: &i2gw.ProviderConf{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewProvider(tt.conf); got == nil {
				t.Errorf("NewProvider() = %v, want non-nil", got)
			}
		})
	}
}
