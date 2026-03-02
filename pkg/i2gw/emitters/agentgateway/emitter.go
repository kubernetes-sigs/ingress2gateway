/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agentgateway

import (
	"sort"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	emitterName      = "agentgateway"
	gatewayClassName = "agentgateway"
)

func init() {
	i2gw.EmitterConstructorByName[emitterName] = NewEmitter
}

type Emitter struct {
	builderMap *BuilderMap
}

// NewEmitter returns a new instance of AgentgatewayEmitter.
func NewEmitter(_ *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{builderMap: NewBuilderMap()}
}

// Emit converts EmitterIR to Gateway API resources plus agentgateway-specific extensions.
func (e *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return gatewayResources, errs
	}

	for key := range gatewayResources.Gateways {
		gw := gatewayResources.Gateways[key]
		gw.Spec.GatewayClassName = gatewayClassName
		gatewayResources.Gateways[key] = gw
	}

	e.EmitBuffer(ir)

	var agentgatewayObjs []client.Object
	for _, policy := range e.builderMap.Policies {
		agentgatewayObjs = append(agentgatewayObjs, policy)
	}

	sort.SliceStable(agentgatewayObjs, func(i, j int) bool {
		oi, oj := agentgatewayObjs[i], agentgatewayObjs[j]
		gvkI := oi.GetObjectKind().GroupVersionKind()
		gvkJ := oj.GetObjectKind().GroupVersionKind()

		if gvkI.Kind != gvkJ.Kind {
			return gvkI.Kind < gvkJ.Kind
		}
		if oi.GetNamespace() != oj.GetNamespace() {
			return oi.GetNamespace() < oj.GetNamespace()
		}
		return oi.GetName() < oj.GetName()
	})

	for _, obj := range agentgatewayObjs {
		u, err := i2gw.CastToUnstructured(obj)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("emitter", emitterName, "AgentgatewayPolicy"), obj.GetName(), err.Error()))
			continue
		}
		gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *u)
	}

	return gatewayResources, errs
}
