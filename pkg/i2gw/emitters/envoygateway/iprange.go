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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/samber/lo"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (e *Emitter) EmitIPRangeControl(ir emitterir.EmitterIR, gwResources *i2gw.GatewayResources) {
	for nn, ctx := range ir.HTTPRoutes {
		if ctx.IPRangeControlByRuleIdx == nil {
			continue
		}

		MergeIPRangeControlIR(&ctx)

		for idx, ipCon := range ctx.IPRangeControlByRuleIdx {
			var sectionName *gwapiv1.SectionName
			if idx != RouteRuleAllIndex && idx < len(ctx.Spec.Rules) {
				sectionName = ctx.Spec.Rules[idx].Name
			}

			securityPolicy := e.getOrBuildSecurityPolicy(ctx, sectionName, idx)

			if len(ipCon.AllowList) > 0 && len(ipCon.DenyList) > 0 {
				securityPolicy.Spec.Authorization = &egapiv1a1.Authorization{
					DefaultAction: ptr.To(egapiv1a1.AuthorizationActionDeny),
					Rules: []egapiv1a1.AuthorizationRule{
						{
							Action: egapiv1a1.AuthorizationActionDeny,
							Principal: egapiv1a1.Principal{
								ClientCIDRs: lo.Map(ipCon.DenyList, func(a string, _ int) egapiv1a1.CIDR {
									return egapiv1a1.CIDR(a)
								}),
							},
						},
						{
							Action: egapiv1a1.AuthorizationActionAllow,
							Principal: egapiv1a1.Principal{
								ClientCIDRs: lo.Map(ipCon.AllowList, func(a string, _ int) egapiv1a1.CIDR {
									return egapiv1a1.CIDR(a)
								}),
							},
						},
					},
				}
			} else if len(ipCon.AllowList) > 0 {
				securityPolicy.Spec.Authorization = &egapiv1a1.Authorization{
					DefaultAction: ptr.To(egapiv1a1.AuthorizationActionDeny),
					Rules: []egapiv1a1.AuthorizationRule{
						{
							Action: egapiv1a1.AuthorizationActionAllow,
							Principal: egapiv1a1.Principal{
								ClientCIDRs: lo.Map(ipCon.AllowList, func(a string, _ int) egapiv1a1.CIDR {
									return egapiv1a1.CIDR(a)
								}),
							},
						},
					},
				}
			} else if len(ipCon.DenyList) > 0 {
				securityPolicy.Spec.Authorization = &egapiv1a1.Authorization{
					DefaultAction: ptr.To(egapiv1a1.AuthorizationActionAllow),
					Rules: []egapiv1a1.AuthorizationRule{
						{
							Action: egapiv1a1.AuthorizationActionDeny,
							Principal: egapiv1a1.Principal{
								ClientCIDRs: lo.Map(ipCon.DenyList, func(a string, _ int) egapiv1a1.CIDR {
									return egapiv1a1.CIDR(a)
								}),
							},
						},
					},
				}
			}

			ruleInfo := ""
			if sectionName != nil {
				ruleInfo = fmt.Sprintf(" rule %s", *sectionName)
			}
			notify(notifications.InfoNotification, fmt.Sprintf("applied IP Range Control feature for HTTPRoute%s", ruleInfo), &ctx.HTTPRoute)
		}

		// mark IP Range Control IR as processed
		ctx.IPRangeControlByRuleIdx = nil
		ir.HTTPRoutes[nn] = ctx
	}
}
