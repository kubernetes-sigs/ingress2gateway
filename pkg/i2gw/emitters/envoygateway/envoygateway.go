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

package envoygateway_emitter

import (
	"fmt"

	egv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const emitterName = "envoy-gateway"

func init() {
	i2gw.EmitterConstructorByName[emitterName] = NewEmitter
}

type Emitter struct{}

func NewEmitter(_ *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{}
}

func (c *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	for key, ctx := range ir.HTTPRoutes {
		utils.MergeExternalAuth(&ctx)
		ir.HTTPRoutes[key] = ctx
	}

	// NOTE:
	// If common emmiter will implement, should remove `utils.ToGatewayResources`.
	// Envoy Gateway Emitter should only handle custom resources generation.
	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	if errs := c.toCustomResources(ir, &gatewayResources); len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	return gatewayResources, nil
}

func (c *Emitter) toCustomResources(ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	c.buildExternalAuth(ir, gatewayResources)
	return nil
}

func (c *Emitter) buildExternalAuth(ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) {
	for httpRouteKey, httpRouteCtx := range ir.HTTPRoutes {
		securityPolicies := addSecurityPolicyIfConfigured(httpRouteKey, &httpRouteCtx)
		if len(securityPolicies) == 0 {
			continue
		}
		for _, securityPolicy := range securityPolicies {
			obj, err := i2gw.CastToUnstructured(securityPolicy)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast SecurityPolicy to unstructured", securityPolicy)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}
	}
}

func addSecurityPolicyIfConfigured(httpRouteNN types.NamespacedName, httpRouteIR *emitterir.HTTPRouteContext) []*egv1a1.SecurityPolicy {
	if httpRouteIR == nil || httpRouteIR.ExtAuth == nil {
		return nil
	}
	securityPolicies := make([]*egv1a1.SecurityPolicy, 0, len(httpRouteIR.ExtAuth))

	for extAuthKey, extAuthConfig := range httpRouteIR.ExtAuth {
		if extAuthKey == emitterir.RouteAllRulesKey {
			securityPolicy := buildSecurityPolicy(httpRouteNN, nil)
			securityPolicy.Spec.ExtAuth = buildSecurityPolicyExtAuth(extAuthConfig)

			securityPolicies = append(securityPolicies, securityPolicy)
		} else {
			// TODO [kkk777-7]: implement per-rule external auth emitting.
			// Currently, ingress2gateway output doesn't contains rule name in HTTPRoute.
			// If will support output rule name, can attach section name to SecurityPolicy.
			notify(notifications.WarningNotification,
				fmt.Sprintf("Failed to emit external Auth for HTTPRoute %s/%s (rule %d), not support emitting policy for per-rule",
					httpRouteNN.Namespace, httpRouteNN.Name, extAuthKey),
				&httpRouteIR.HTTPRoute,
			)
		}
	}
	return securityPolicies
}
