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

package envoygateway

import (
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/gvk"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	egv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// The Name of the provider.
const Name = "envoy-gateway"

// Emitter implements the i2gw.Emitter interface.
type Emitter struct {
	common common.Emitter
}

type emittedResources struct {
	backendTrafficPolicies map[types.NamespacedName]*egv1a1.BackendTrafficPolicy
}

var _ i2gw.Emitter = &Emitter{}

// init registers the emitter as the envoy-gateway emitter in the registry.
func init() {
	i2gw.EmitterByName[i2gw.EmitterName(Name)] = NewEmitter()
}

func NewEmitter() i2gw.Emitter {
	return &Emitter{common.Emitter{}}
}

func (e *Emitter) ToGatewayResources(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errorList := e.common.ToGatewayResources(ir)
	if len(errorList) > 0 {
		return gatewayResources, errorList
	}
	errorList = e.toProviderResources(ir, &gatewayResources)

	return gatewayResources, errorList
}

func (e *Emitter) toProviderResources(ir intermediate.IR, gwResources *i2gw.GatewayResources) field.ErrorList {
	emittedResources := &emittedResources{
		backendTrafficPolicies: make(map[types.NamespacedName]*egv1a1.BackendTrafficPolicy),
	}
	errorList := e.bufferToProviderResource(emittedResources, ir)

	for btpKey, btp := range emittedResources.backendTrafficPolicies {
		obj, err := i2gw.CastToUnstructured(btp)
		if err != nil {
			errorList = append(errorList, field.InternalError(
				field.NewPath("backendtrafficpolicy", btpKey.Namespace, btpKey.Name),
				fmt.Errorf("failed to convert BackendTrafficPolicy to unstructured: %w", err),
			))
			continue
		}
		gwResources.GatewayExtensions = append(gwResources.GatewayExtensions, *obj)
	}
	return errorList
}

func (e *Emitter) bufferToProviderResource(
	emittedResources *emittedResources,
	ir intermediate.IR,
) field.ErrorList {
	var errs field.ErrorList

	for httpRouteKey, httpRouteContext := range ir.HTTPRoutes {
		settings := httpRouteContext.ExtensionSettings
		if len(settings) == 0 {
			continue
		}

		allAttachedKey := intermediate.RouteSettingsAttachment{
			Rule:    intermediate.IndexAttachAllRules,
			Backend: intermediate.IndexAttachAllBackends,
		}

		// Check if the settings apply to all rules
		if setting, ok := settings[allAttachedKey]; ok && setting.Buffer != nil {
			btpNN := types.NamespacedName{
				Name:      httpRouteKey.Name,
				Namespace: httpRouteKey.Namespace,
			}

			btp, exists := emittedResources.backendTrafficPolicies[btpNN]
			if !exists {
				btp = &egv1a1.BackendTrafficPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      btpNN.Name,
						Namespace: btpNN.Namespace,
						Annotations: map[string]string{
							i2gw.GeneratorAnnotationKey: fmt.Sprintf("ingress2gateway-%s", i2gw.Version),
						},
					},
					Spec: egv1a1.BackendTrafficPolicySpec{
						PolicyTargetReferences: egv1a1.PolicyTargetReferences{
							TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
								{
									LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
										Group: gatewayv1.Group(gvk.HTTPRouteGVK.Group),
										Kind:  gatewayv1.Kind(gvk.HTTPRouteGVK.Kind),
										Name:  gatewayv1.ObjectName(httpRouteKey.Name),
									},
								},
							},
						},
					},
				}
				btp.SetGroupVersionKind(BackendTrafficPolicyGVK)
				emittedResources.backendTrafficPolicies[btpNN] = btp
			}

			btp.Spec.RequestBuffer = &egv1a1.RequestBuffer{
				Limit: *setting.Buffer,
			}

			notify(notifications.InfoNotification,
				fmt.Sprintf("generated BackendTrafficPolicy with buffer size %s for HTTPRoute %s/%s",
					setting.Buffer.String(), httpRouteKey.Namespace, httpRouteKey.Name),
				btp)
		} else {
			// TODO [kkk777-7]: Should handle per route rule name (sectionName) attachments.
			// This would enable per-rule buffer configuration in a merged HTTPRoute, even when
			// multiple Ingresses with different buffer sizes are consolidated into a single HTTPRoute.
			// We can't support this until common HTTPRoute output supports route rule names.

			// Check if there are per-rule settings that we don't support yet
			for attachment, setting := range settings {
				if setting.Buffer != nil {
					notify(notifications.WarningNotification,
						fmt.Sprintf("per-rule buffer configuration is not supported yet for HTTPRoute %s/%s (rule %d), skipping",
							httpRouteKey.Namespace, httpRouteKey.Name, attachment.Rule),
						&httpRouteContext.HTTPRoute)
					break
				}
			}
		}
	}
	return errs
}
