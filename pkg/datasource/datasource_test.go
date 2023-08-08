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

package datasource

import (
	"regexp"
	"testing"
)

func Test_getIngressList(t *testing.T) {
	testCases := []struct {
		name       string
		ds         DataSource
		wantErrMsg string
	}{{
		name: "Test input file does not exist",
		ds: DataSource{
			InputFile:       "non-existing-file",
			NamespaceFilter: "",
		},
		wantErrMsg: "failed to open input file",
	}, {
		name: "Test input file does not have resources for the given namespace",
		ds: DataSource{
			InputFile:       "testdata/input-file.yaml",
			NamespaceFilter: "non-existing-namespace",
		},
		wantErrMsg: "No resources found",
	},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.ds.GetIngessList()
			if err == nil {
				t.Errorf("Expected Error to contain the following substring: %+v\n Got no error", tc.wantErrMsg)
			}
			match, _ := regexp.MatchString(tc.wantErrMsg, err.Error())
			if !match {
				t.Errorf("Expected Error to contain the following substring: %+v\n Got: %+v\n", tc.wantErrMsg, err.Error())
			}
		})
	}
}
