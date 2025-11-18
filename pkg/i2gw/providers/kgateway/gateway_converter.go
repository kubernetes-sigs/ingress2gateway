package kgateway

import (
	kgwv1a1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type irToGatewayResourcesConverter struct{}

// newIRToGatewayResourcesConverter returns a kgateway irToGatewayResourcesConverter instance.
func newIRToGatewayResourcesConverter() irToGatewayResourcesConverter {
	return irToGatewayResourcesConverter{}
}

func (c *irToGatewayResourcesConverter) irToGateway(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := common.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	buildTrafficPolicies(ir, &gatewayResources)
	return gatewayResources, nil
}

func buildTrafficPolicies(ir intermediate.IR, gatewayResources *i2gw.GatewayResources) {
	for httpRouteKey, httpRouteContext := range ir.HTTPRoutes {
		kgw := httpRouteContext.ProviderSpecificIR.Kgateway
		if kgw == nil {
			continue
		}
		tp := map[string]*kgwv1a1.TrafficPolicy{}
		createIfNeeded := func(s string) {
			if tp[s] == nil {
				tp[s] = &kgwv1a1.TrafficPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      s,
						Namespace: httpRouteKey.Namespace,
					},
					Spec: kgwv1a1.TrafficPolicySpec{},
				}
				tp[s].SetGroupVersionKind(TrafficPolicyGVK)
			}
		}

		for polSourceIngressName, pol := range kgw.Policies {
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
			// if the entire http route is covered by this policy, set targetRef to the http route
			if len(pol.RuleBackendSources) == numRules(httpRouteContext.HTTPRoute) {
				t.Spec.TargetRefs = []kgwv1a1.LocalPolicyTargetReferenceWithSectionName{{
					LocalPolicyTargetReference: kgwv1a1.LocalPolicyTargetReference{
						Name: gatewayv1.ObjectName(httpRouteKey.Name),
					},
				}}
			} else {
				// otherwise this httprule is combined from multiple ingress objects. use extensionRefs
				// filters to point to the traffic policy from the relevant rules/backends
				for _, idx := range pol.RuleBackendSources {
					httpRouteContext.Spec.Rules[idx.Rule].BackendRefs[idx.Backend].Filters = append(httpRouteContext.Spec.Rules[idx.Rule].BackendRefs[idx.Backend].Filters, gatewayv1.HTTPRouteFilter{
						Type: gatewayv1.HTTPRouteFilterExtensionRef,
						ExtensionRef: &gatewayv1.LocalObjectReference{
							Group: gatewayv1.Group(TrafficPolicyGVK.Group),
							Kind:  gatewayv1.Kind(TrafficPolicyGVK.Kind),
							Name:  gatewayv1.ObjectName(t.Name),
						},
					})
				}
			}
		}

		for _, tp := range tp {
			obj, err := i2gw.CastToUnstructured(tp)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast TrafficPolicy to unstructured", tp)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}
	}
}

func numRules(hr gatewayv1.HTTPRoute) int {
	numRules := 0
	for _, rule := range hr.Spec.Rules {
		numRules += len(rule.BackendRefs)
	}
	return numRules
}
