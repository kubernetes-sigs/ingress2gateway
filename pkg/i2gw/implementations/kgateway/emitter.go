/*
Copyright 2024 The Kubernetes Authors.

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

package kgateway

import (
	kgwv1a1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	Name = "kgateway"
)

// KgatewayEmitter implements ImplementationEmitter and generates Kgateway
// resources from the merged IR, using provider output as the source.
type KgatewayEmitter struct{}

// Ensure KgatewayEmitter satisfies the ImplementationEmitter interface.
var _ i2gw.ImplementationEmitter = &KgatewayEmitter{}

func init() {
	// Register the kgateway emitter.
	i2gw.ImplementationEmitters[Name] = NewKgatewayEmitter()
}

// NewKgatewayEmitter returns a new instance of KgatewayEmitter.
func NewKgatewayEmitter() i2gw.ImplementationEmitter {
	return &KgatewayEmitter{}
}

// Name returns the name of the kgateway implementation.
func (e *KgatewayEmitter) Name() string {
	return Name
}

// Emit consumes the IR and returns Kgateway-specific resources as client.Objects.
// This implementation treats providers (e.g. ingress-nginx) as sources that populate
// provider-specific IR. It reads generic Policies from the provider IR and turns them
// into Kgateway resources and/or mutates the given IR. Whole-route policies are attached
// via targetRefs; partial policies are attached as ExtensionRef filters on backendRefs.
func (e *KgatewayEmitter) Emit(ir *intermediate.IR) ([]client.Object, error) {
	var out []client.Object
	var errs field.ErrorList

	for httpRouteKey, httpRouteContext := range ir.HTTPRoutes {
		ingx := httpRouteContext.ProviderSpecificIR.IngressNginx
		if ingx == nil {
			continue
		}

		tp := map[string]*kgwv1a1.TrafficPolicy{}
		createIfNeeded := func(name string) {
			if tp[name] == nil {
				tp[name] = &kgwv1a1.TrafficPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: httpRouteKey.Namespace,
					},
					Spec: kgwv1a1.TrafficPolicySpec{},
				}
				tp[name].SetGroupVersionKind(TrafficPolicyGVK)
			}
		}

		for polSourceIngressName, pol := range ingx.Policies {
			var t *kgwv1a1.TrafficPolicy
			if pol.Buffer != nil {
				createIfNeeded(polSourceIngressName)
				t = tp[polSourceIngressName]
				t.Spec.Buffer = &kgwv1a1.Buffer{
					MaxRequestSize: pol.Buffer,
				}
			}
			if t == nil {
				continue
			}

			if len(pol.RuleBackendSources) == numRules(httpRouteContext.HTTPRoute) {
				// Full coverage via targetRefs.
				t.Spec.TargetRefs = []kgwv1a1.LocalPolicyTargetReferenceWithSectionName{{
					LocalPolicyTargetReference: kgwv1a1.LocalPolicyTargetReference{
						Name: gwv1.ObjectName(httpRouteKey.Name),
					},
				}}
			} else {
				// Partial coverage via ExtensionRef filters on backendRefs.
				for _, idx := range pol.RuleBackendSources {
					httpRouteContext.Spec.Rules[idx.Rule].BackendRefs[idx.Backend].Filters =
						append(
							httpRouteContext.Spec.Rules[idx.Rule].BackendRefs[idx.Backend].Filters,
							gwv1.HTTPRouteFilter{
								Type: gwv1.HTTPRouteFilterExtensionRef,
								ExtensionRef: &gwv1.LocalObjectReference{
									Group: gwv1.Group(TrafficPolicyGVK.Group),
									Kind:  gwv1.Kind(TrafficPolicyGVK.Kind),
									Name:  gwv1.ObjectName(t.Name),
								},
							},
						)
				}
			}
		}

		// Write back the mutated HTTPRouteContext into the IR.
		ir.HTTPRoutes[httpRouteKey] = httpRouteContext

		// Collect TrafficPolicies.
		for _, tp := range tp {
			out = append(out, tp)
		}
	}

	if len(errs) > 0 {
		return out, i2gw.AggregatedErrs(errs)
	}
	return out, nil
}

func numRules(hr gwv1.HTTPRoute) int {
	n := 0
	for _, r := range hr.Spec.Rules {
		n += len(r.BackendRefs)
	}
	return n
}
