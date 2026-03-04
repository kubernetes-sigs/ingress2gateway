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

package agentgateway

import (
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1/shared"
)

// applyAccessLogPolicy projects ingress-nginx enable-access-log into AgentgatewayPolicy.spec.frontend.accessLog.
//
// Semantics:
//   - true: emit frontend.accessLog with default settings (logs enabled).
//   - false: emit frontend.accessLog.filter="false" (logs disabled explicitly).
func applyAccessLogPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.EnableAccessLog == nil {
		return false
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Frontend == nil {
		agp.Spec.Frontend = &agentgatewayv1alpha1.Frontend{}
	}
	if agp.Spec.Frontend.AccessLog == nil {
		agp.Spec.Frontend.AccessLog = &agentgatewayv1alpha1.AccessLog{}
	}

	if *pol.EnableAccessLog {
		// Explicitly clear filter in case another merge path had disabled logs.
		agp.Spec.Frontend.AccessLog.Filter = nil
		return true
	}

	disableAll := shared.CELExpression("false")
	agp.Spec.Frontend.AccessLog.Filter = &disableAll
	return true
}
