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

package nginx

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const emitterName = "nginx"

func init() {
	i2gw.EmitterConstructorByName[emitterName] = NewEmitter
}

type Emitter struct {
	builderMap *BuilderMap
	notify     notifications.NotifyFunc
}

// NewEmitter returns a new instance of an NGINX emitter
func NewEmitter(conf *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{
		builderMap: NewBuilderMap(),
		notify:     conf.Report.Notifier(emitterName),
	}
}

// Emit converts emitter IR to Gateway API and NGINX specific resources
func (e *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	utils.AddHTTPRouteRuleNames(ir)

	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}

	// Set GatewayClassName to NGINX for all the gateways
	for key := range gatewayResources.Gateways {
		gateway := gatewayResources.Gateways[key]
		gateway.Spec.GatewayClassName = emitterName
		gatewayResources.Gateways[key] = gateway
	}

	e.ToNginxResources(ir, &gatewayResources)

	utils.LogUnparsedErrors(ir, e.notify)
	return gatewayResources, nil
}

// ToNginxResources processes emitterIR and adds nginx-specific extensions to gatewayResources
func (e *Emitter) ToNginxResources(ir emitterir.EmitterIR, gwResources *i2gw.GatewayResources) {
	e.emitClientSettingsPolicies(ir, gwResources)
}
