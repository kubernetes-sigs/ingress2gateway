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

package default_emitter

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/common"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func init() {
	i2gw.EmitterConstructorByName["default"] = NewDefaultEmitter
}

type DefaultEmitter struct{}

// DefaultEmitter is the default emitter that converts the intermediate representation
// to Gateway API resources without any provider-specific modifications.
func NewDefaultEmitter(_ *i2gw.EmitterConf) i2gw.Emitter {
	return &DefaultEmitter{}
}

// Emit converts the provider intermediate representation to Gateway API resources.
func (e *DefaultEmitter) Emit(ir emitter_intermediate.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	return common.ToGatewayResources(ir)
}
