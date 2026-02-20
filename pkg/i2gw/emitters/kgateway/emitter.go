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

package kgateway

import (
	"sort"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	i2gw.EmitterConstructorByName["kgateway"] = NewEmitter
}

type Emitter struct {
	builderMap *BuilderMap
}

// NewEmitter returns a new instance of KgatewayEmitter
func NewEmitter(_ *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{
		builderMap: NewBuilderMap(),
	}
}

// Emit converts EmitterIR to Gateway API resources plus kgateway-specific extensions
func (e *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}

	// Set GatewayClassName to "kgateway" for all Gateways
	for key := range gatewayResources.Gateways {
		gateway := gatewayResources.Gateways[key]
		gateway.Spec.GatewayClassName = "kgateway"
		gatewayResources.Gateways[key] = gateway
	}

	e.ToKgatewayResources(ir, &gatewayResources)

	return gatewayResources, nil
}

// ToKgatewayResources processes emitterIR and adds kgateway-specific extensions to gatewayResources
func (e *Emitter) ToKgatewayResources(ir emitterir.EmitterIR, gwResources *i2gw.GatewayResources) {
	e.EmitBuffer(ir)

	// Collect all TrafficPolicies and convert to unstructured
	var kgatewayObjs []client.Object
	for _, trafficPolicy := range e.builderMap.TrafficPolicies {
		kgatewayObjs = append(kgatewayObjs, trafficPolicy)
	}

	// Sort by Kind, then Namespace, then Name to make output deterministic for testing
	sort.SliceStable(kgatewayObjs, func(i, j int) bool {
		oi, oj := kgatewayObjs[i], kgatewayObjs[j]

		gvki := oi.GetObjectKind().GroupVersionKind()
		gvkj := oj.GetObjectKind().GroupVersionKind()

		ki, kj := gvki.Kind, gvkj.Kind
		if ki != kj {
			return ki < kj
		}

		nsi, nsj := oi.GetNamespace(), oj.GetNamespace()
		if nsi != nsj {
			return nsi < nsj
		}

		return oi.GetName() < oj.GetName()
	})

	// Convert kgateway objects to unstructured and add to GatewayExtensions
	for _, obj := range kgatewayObjs {
		u, err := i2gw.CastToUnstructured(obj)
		if err != nil {
			notify(notifications.ErrorNotification, "Failed to cast TrafficPolicy to unstructured", obj)
			continue
		}
		gwResources.GatewayExtensions = append(gwResources.GatewayExtensions, *u)
	}
}
