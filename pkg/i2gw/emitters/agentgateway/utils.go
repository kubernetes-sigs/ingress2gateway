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

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// routeRuleAllIndex is used to indicate a policy applies to all rules in an HTTPRoute.
	routeRuleAllIndex = -1
)

func lookupSectionName(rc *emitterir.HTTPRouteContext, idx int) *gatewayv1.SectionName {
	if idx != routeRuleAllIndex && idx < len(rc.Spec.Rules) {
		return rc.Spec.Rules[idx].Name
	}
	return nil
}

func formatRuleInfo(rc *emitterir.HTTPRouteContext, idx int) string {
	if name := lookupSectionName(rc, idx); name != nil {
		return fmt.Sprintf(" rule %s", *name)
	}
	return ""
}
