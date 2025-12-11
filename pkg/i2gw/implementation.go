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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ImplementationEmitter generates GEP-713 style implementation-specific resources
// from the merged IR.
type ImplementationEmitter interface {
	// Name of the implementation.
	Name() string

	// Emit can mutate the IR (e.g. to inject HTTPRoute filters) and returns
	// implementation-specific objects.
	Emit(ir *intermediate.IR) ([]client.Object, error)
}

// ImplementationEmitters defines a global registry, similar to ProviderConstructorByName.
var ImplementationEmitters = map[string]ImplementationEmitter{}
