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
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

type storage struct {
	mu sync.RWMutex

	resources []*openapi3.T
}

func newResourceStorage() *storage {
	return &storage{}
}

func (s *storage) addResource(resource *openapi3.T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resources = append(s.resources, resource)
}

func (s *storage) getResources() []*openapi3.T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.resources
}
