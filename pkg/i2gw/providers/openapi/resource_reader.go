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

package openapi

import (
	"context"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a reader instance.
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	return &resourceReader{
		conf: conf,
	}
}

func (r *resourceReader) readResourcesFromFile(ctx context.Context, filename string) (*storage, error) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	if err := spec.Validate(ctx); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI 3.x spec: %w", err)
	}

	storage := newResourceStorage()
	storage.addResource(spec)

	return storage, nil
}
