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

package kgateway_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.yaml.in/yaml/v4"
)

// getModuleRoot finds the repo root by asking Go where go.mod lives.
func getModuleRoot(t *testing.T) string {
	t.Helper()

	cmd := exec.Command("go", "env", "GOMOD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to run 'go env GOMOD': %v", err)
	}
	goModPath := strings.TrimSpace(string(out))
	if goModPath == "" {
		t.Fatalf("go env GOMOD returned empty path")
	}
	return filepath.Dir(goModPath)
}

func canonicalizeMultiDocYAML(t *testing.T, in []byte) []byte {
	t.Helper()

	// Split multi-doc YAML into individual docs.
	dec := yaml.NewDecoder(bytes.NewReader(in))

	type doc struct {
		Raw map[string]any
	}
	var docs []doc

	for {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("failed to decode yaml: %v", err)
		}
		if len(m) == 0 {
			continue
		}
		docs = append(docs, doc{Raw: m})
	}

	// Extract Kind/Name for sorting.
	type keyedDoc struct {
		kind      string
		namespace string
		name      string
		raw       map[string]any
	}

	var kd []keyedDoc
	for _, d := range docs {
		md, _ := d.Raw["metadata"].(map[string]any)
		kind, _ := d.Raw["kind"].(string)
		name, _ := md["name"].(string)
		ns, _ := md["namespace"].(string)
		kd = append(kd, keyedDoc{
			kind:      kind,
			namespace: ns,
			name:      name,
			raw:       d.Raw,
		})
	}

	sort.Slice(kd, func(i, j int) bool {
		if kd[i].kind != kd[j].kind {
			return kd[i].kind < kd[j].kind
		}
		if kd[i].namespace != kd[j].namespace {
			return kd[i].namespace < kd[j].namespace
		}
		return kd[i].name < kd[j].name
	})

	// Re-encode in canonical order.
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	for _, d := range kd {
		if err := enc.Encode(d.raw); err != nil {
			t.Fatalf("failed to encode yaml: %v", err)
		}
	}
	_ = enc.Close()

	return buf.Bytes()
}

func TestKgatewayIngressNginxIntegration_Golden(t *testing.T) {
	t.Helper()

	moduleRoot := getModuleRoot(t)

	inputPath := filepath.Join(
		moduleRoot,
		"pkg", "i2gw", "implementations", "kgateway", "testing", "testdata", "input.yaml",
	)
	goldenPath := filepath.Join(
		moduleRoot,
		"pkg", "i2gw", "implementations", "kgateway", "testing", "testdata", "output.yaml",
	)

	cmd := exec.Command(
		"go", "run", ".",
		"print",
		"--providers=ingress-nginx",
		"--implementations=kgateway",
		"--input-file", inputPath,
	)
	cmd.Dir = moduleRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("ingress2gateway command failed: %v\nstderr:\n%s", err, stderr.String())
	}

	actual := stdout.Bytes()

	// Golden file handling
	writeGolden := false
	goldenBytes, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		writeGolden = true
	} else if err != nil {
		t.Fatalf("failed to read golden file %q: %v", goldenPath, err)
	}

	if os.Getenv("REFRESH_GOLDEN") == "true" {
		writeGolden = true
	}

	if writeGolden {
		if err := os.WriteFile(goldenPath, actual, 0o600); err != nil {
			t.Fatalf("failed to write golden file %q: %v", goldenPath, err)
		}
		t.Logf("wrote golden file: %s", goldenPath)
		return
	}

	// Normalize trivial formatting differences
	actualTrimmed := bytes.TrimSpace(actual)
	expectedTrimmed := bytes.TrimSpace(goldenBytes)

	got := canonicalizeMultiDocYAML(t, actualTrimmed)
	want := canonicalizeMultiDocYAML(t, expectedTrimmed)

	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Fatalf("golden output mismatch (-want +got):\n%s", diff)
	}
}
