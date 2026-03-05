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

package standard_emitter

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const emitterName = "standard_emitter"

func init() {
	i2gw.EmitterConstructorByName["standard"] = NewEmitter
}

type Emitter struct {
	notify notifications.NotifyFunc
}

// Emitter is the standard emitter that converts the intermediate representation
// to Gateway API resources without any provider-specific modifications.
func NewEmitter(conf *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{
		notify: conf.Report.Notifier(emitterName),
	}
}

func (e *Emitter) regexWarnings(gwResources *i2gw.GatewayResources) {
	for _, route := range gwResources.HTTPRoutes {
		hasRegex := false
		hasNonRegex := false
		for _, rule := range route.Spec.Rules {
			if len(rule.Matches) == 0 {
				hasNonRegex = true
			}
			for _, match := range rule.Matches {
				if match.Path != nil && match.Path.Type != nil && *match.Path.Type == "RegularExpression" {
					hasRegex = true
				} else {
					hasNonRegex = true
				}
			}
		}
		if hasRegex && hasNonRegex {
			e.notify(notifications.WarningNotification, "HTTPRoute contains both regex and non-regex path matches, which have implementation-specific priorities", &route)
		}
	}

}

// Emit converts the provider intermediate representation to Gateway API resources.
func (e *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	utils.LogUnparsedErrors(ir, e.notify)
	resources, err := utils.ToGatewayResources(ir)
	e.regexWarnings(&resources)
	return resources, err
}
