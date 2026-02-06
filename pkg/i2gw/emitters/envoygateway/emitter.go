/*
Copyright 2026 The Kubernetes Authors.

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

package envoygateway_emitter

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

const emitterName = "envoy-gateway"

func init() {
	i2gw.EmitterConstructorByName[emitterName] = NewEmitter
}

type Emitter struct {
	builderMap *BuilderMap
}

func NewEmitter(_ *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{
		builderMap: NewBuilderMap(),
	}
}

func (e *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	e.ToEnvoyGatewayResources(ir, &gatewayResources)

	return gatewayResources, nil
}

func (e *Emitter) ToEnvoyGatewayResources(ir emitterir.EmitterIR, gwResources *i2gw.GatewayResources) {
	e.EmitBuffer(ir, gwResources)
	e.EmitCors(ir, gwResources)

	for _, backendTrafficPolicy := range e.builderMap.BackendTrafficPolicies {
		obj, err := i2gw.CastToUnstructured(backendTrafficPolicy)
		if err != nil {
			notify(notifications.ErrorNotification, "Failed to cast BackendTrafficPolicy to unstructured", backendTrafficPolicy)
			continue
		}
		gwResources.GatewayExtensions = append(gwResources.GatewayExtensions, *obj)
	}

	for _, securityPolicy := range e.builderMap.SecurityPolicies {
		obj, err := i2gw.CastToUnstructured(securityPolicy)
		if err != nil {
			notify(notifications.ErrorNotification, "Failed to cast SecurityPolicy to unstructured", securityPolicy)
			continue
		}
		gwResources.GatewayExtensions = append(gwResources.GatewayExtensions, *obj)
	}
}
