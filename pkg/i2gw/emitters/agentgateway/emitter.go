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

package agentgateway

import (
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	emitterName      = "agentgateway"
	gatewayClassName = "agentgateway"
)

func init() {
	i2gw.EmitterConstructorByName[emitterName] = NewEmitter
}

type Emitter struct{}

// NewEmitter returns a new instance of the Agentgateway emitter.
func NewEmitter(_ *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{}
}

// Emit converts EmitterIR to Gateway API resources.
//
// Today, the agentgateway emitter is intentionally minimal:
//   - it emits the standard Gateway API resources
//   - it sets GatewayClassName="agentgateway" on all Gateways
//
// As upstream EmitterIR grows new provider-neutral intents, this emitter can be
// extended to emit agentgateway-specific extensions.
func (e *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	gwResources, errs := utils.ToGatewayResources(ir)
	if errs != nil {
		return gwResources, errs
	}

	// Set GatewayClassName to "agentgateway" for all Gateways.
	for key, gw := range gwResources.Gateways {
		gw.Spec.GatewayClassName = gatewayClassName
		gwResources.Gateways[key] = gw
	}

	return gwResources, nil
}
