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
	"fmt"

	egapiv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/samber/lo"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

func (e *Emitter) EmitCors(ir emitterir.EmitterIR, gwResources *i2gw.GatewayResources) {
	for nn, ctx := range ir.HTTPRoutes {
		MergeCorsIR(&ctx)

		for idx, cors := range ctx.CorsPolicyByRuleIdx {
			// if CORS is nil, it means that CORS IR has already applied by common emitter.
			if cors == nil {
				continue
			}

			var sectionName *gwapiv1.SectionName
			if idx != RouteRuleAllIndex && idx < len(ctx.Spec.Rules) {
				sectionName = ctx.Spec.Rules[idx].Name
			}

			securityPolicy := e.getOrBuildSecurityPolicy(ctx, sectionName, idx)
			if securityPolicy.Spec.CORS == nil {
				securityPolicy.Spec.CORS = &egapiv1a1.CORS{}
			}
			securityPolicy.Spec.CORS.AllowOrigins = lo.Map(cors.AllowOrigins, func(origin gwapiv1.CORSOrigin, _ int) egapiv1a1.Origin {
				return egapiv1a1.Origin(origin)
			})
			securityPolicy.Spec.CORS.AllowMethods = lo.Map(cors.AllowMethods, func(method gwapiv1.HTTPMethodWithWildcard, _ int) string {
				return string(method)
			})
			securityPolicy.Spec.CORS.AllowHeaders = lo.Map(cors.AllowHeaders, func(header gwapiv1.HTTPHeaderName, _ int) string {
				return string(header)
			})
			securityPolicy.Spec.CORS.ExposeHeaders = lo.Map(cors.ExposeHeaders, func(header gwapiv1.HTTPHeaderName, _ int) string {
				return string(header)
			})
			if cors.MaxAge > 0 {
				securityPolicy.Spec.CORS.MaxAge = ptr.To(gwapiv1.Duration(fmt.Sprintf("%ds", cors.MaxAge)))
			}
			securityPolicy.Spec.CORS.AllowCredentials = cors.AllowCredentials

			ruleInfo := ""
			if sectionName != nil {
				ruleInfo = fmt.Sprintf(" rule %s", *sectionName)
			}
			notify(notifications.InfoNotification, fmt.Sprintf("applied CORS feature for HTTPRoute%s", ruleInfo), &ctx.HTTPRoute)

			// mark CORS IR as processed
			ctx.CorsPolicyByRuleIdx[idx] = nil
		}

		ir.HTTPRoutes[nn] = ctx
	}
}
