/*
Copyright © 2023 Kubernetes Authors

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

package version

import (
	"encoding/json"
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
)

var (
	version     string
	gitCommitID string
)

type Info struct {
	Version     string `json:"version"`
	GitCommitID string `json:"gitCommitID"`
}

func Get() Info {
	return Info{
		Version:     version,
		GitCommitID: gitCommitID,
	}
}

// Print shows the versions of the Ingress2Gateway.
func Print(w io.Writer, format string) error {
	v := Get()
	switch format {
	case "json":
		if versionInfo, err := json.MarshalIndent(v, "", "  "); err == nil {
			fmt.Fprintln(w, string(versionInfo))
		}
	case "yaml":
		if versionInfo, err := yaml.Marshal(v); err == nil {
			fmt.Fprintln(w, string(versionInfo))
		}
	default:
		fmt.Fprintf(w, "VERSION: %s\n", v.Version)
		fmt.Fprintf(w, "GIT_COMMIT_ID: %s\n", v.GitCommitID)
	}

	return nil
}
