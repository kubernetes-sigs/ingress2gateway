/*
Copyright The Kubernetes Authors.

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

package agentgateway_emitter

import (
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	emitterName = "agentgateway"
)

func init() {
	i2gw.EmitterConstructorByName[emitterName] = NewEmitter
}

type Emitter struct {
	notify notifications.NotifyFunc
}

func NewEmitter(conf *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{
		notify: conf.Report.Notifier(emitterName),
	}
}

func (e *Emitter) Emit(ir emitterir.EmitterIR) (gr i2gw.GatewayResources, errs field.ErrorList) {
	for ns, gw := range ir.Gateways {
		gw.Spec.GatewayClassName = emitterName
		ir.Gateways[ns] = gw
	}
	gr, errs = utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return
	}

	for nn, rc := range ir.HTTPRoutes {
		applyBodySize(&rc, e.notify)
		ir.HTTPRoutes[nn] = rc
	}

	utils.LogUnparsedErrors(ir, e.notify)
	return gr, nil
}

func applyBodySize(
	rc *emitterir.HTTPRouteContext,
	notify notifications.NotifyFunc,
) {
	for idx := range rc.BodySizeByRuleIdx {
		notify(
			notifications.WarningNotification,
			fmt.Sprintf("Body size limit is not supported for HTTPRoute targets in AgentgatewayPolicy; ignoring%s", formatRuleInfo(rc, idx)),
			&rc.HTTPRoute,
		)
	}
	rc.BodySizeByRuleIdx = nil
}
