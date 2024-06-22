/*
Copyright 2023 The Kubernetes Authors.

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

package i2gw

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_constructProviders(t *testing.T) {
	supportProviders := []string{"ingress-nginx"}
	for _, provider := range supportProviders {
		ProviderConstructorByName[ProviderName(provider)] = func(_ *ProviderConf) Provider { return nil }
	}
	testCases := []struct {
		name              string
		providers         []string
		expectedProviders []string
		expectedError     error
	}{{
		name:              "Test construct providers with default providers",
		providers:         GetSupportedProviders(),
		expectedProviders: supportProviders,
		expectedError:     nil,
	}, {
		name:              "Test construct providers with provider that not supported",
		providers:         []string{"ingress-nginx", "fake-provider"},
		expectedProviders: []string{},
		expectedError:     fmt.Errorf("%s is not a supported provider", "fake-provider"),
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects([]runtime.Object{}...).Build()
			providerByName, err := constructProviders(&ProviderConf{
				Client: cl,
			}, tc.providers)
			if tc.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tc.expectedError.Error() != err.Error() {
					t.Errorf("The got error '%s' not equal to expected error '%s'", err.Error(), tc.expectedError.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got %v", err)
				}
				if len(tc.expectedProviders) != len(providerByName) {
					t.Errorf("Expected contructed providers num is %d but got %d", len(tc.providers), len(providerByName))
				}
				for _, provider := range tc.expectedProviders {
					if _, ok := providerByName[ProviderName(provider)]; !ok {
						t.Errorf("The expected provider %s was not constructed", provider)
					}
				}
			}
		})
	}
}

func Test_GetSupportedProviders(t *testing.T) {
	supportProviders := []string{"ingress-nginx"}
	for _, provider := range supportProviders {
		ProviderConstructorByName[ProviderName(provider)] = func(_ *ProviderConf) Provider { return nil }
	}
	t.Run("Test GetSupportedProviders", func(t *testing.T) {
		allProviders := GetSupportedProviders()
		if len(allProviders) != len(ProviderConstructorByName) {
			t.Errorf("The acutal number of the providers we supported is %d but we got the number is: %d",
				len(ProviderConstructorByName), len(allProviders))
		}
		for _, provider := range allProviders {
			providerName := ProviderName(provider)
			if _, ok := ProviderConstructorByName[providerName]; !ok {
				t.Errorf("%s is not a supported provider", providerName)
			}
		}
	})
}
