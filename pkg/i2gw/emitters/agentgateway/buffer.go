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
	"fmt"
	"math"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	agentgatewayv1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1/agentgateway"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// applyBufferPolicy projects ingress-nginx body-size annotations into
// AgentgatewayPolicy.spec.frontend.http.maxBufferSize.
//
// Semantics:
//   - Prefer proxy-body-size when set.
//   - Otherwise fall back to client-body-buffer-size.
func applyBufferPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) (bool, *field.Error) {
	if pol.ProxyBodySize == nil && pol.ClientBodyBufferSize == nil {
		return false, nil
	}

	size := pol.ProxyBodySize
	if size == nil {
		size = pol.ClientBodyBufferSize
	}
	if size == nil {
		return false, nil
	}

	sizeBytes := size.Value()
	if sizeBytes <= 0 {
		return false, field.Invalid(
			field.NewPath("emitter", "agentgateway", "AgentgatewayPolicy", "frontend", "http", "maxBufferSize"),
			size.String(),
			"resolved maxBufferSize must be greater than 0",
		)
	}
	if sizeBytes > math.MaxInt32 {
		return false, field.Invalid(
			field.NewPath("emitter", "agentgateway", "AgentgatewayPolicy", "frontend", "http", "maxBufferSize"),
			size.String(),
			fmt.Sprintf("resolved maxBufferSize exceeds int32 limit (%d)", int64(math.MaxInt32)),
		)
	}

	maxBufferSize := int32(sizeBytes)
	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Frontend == nil {
		agp.Spec.Frontend = &agentgatewayv1alpha1.Frontend{}
	}
	if agp.Spec.Frontend.HTTP == nil {
		agp.Spec.Frontend.HTTP = &agentgatewayv1alpha1.FrontendHTTP{}
	}
	agp.Spec.Frontend.HTTP.MaxBufferSize = &maxBufferSize
	return true, nil
}
