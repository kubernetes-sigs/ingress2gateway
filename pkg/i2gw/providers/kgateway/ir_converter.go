package kgateway

import (
	"context"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type resourcesToIRConverter struct {
	conf           *i2gw.ProviderConf
	featureParsers []i2gw.FeatureParser

	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
	ctx                           context.Context
	ing2pol                       map[string]intermediate.KgatewayPolicy
}

// newResourcesToIRConverter returns an ingress-kgateway resourcesToIRConverter instance.
func newResourcesToIRConverter(conf *i2gw.ProviderConf) resourcesToIRConverter {

	ing2pol := make(map[string]intermediate.KgatewayPolicy)
	return resourcesToIRConverter{
		conf:                          conf,
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{},
		ctx:                           context.Background(),
		ing2pol:                       ing2pol,
		featureParsers: []i2gw.FeatureParser{
			bufferFeature(ing2pol),
		},
	}
}

func (c *resourcesToIRConverter) convertToIR(storage *storage) (intermediate.IR, field.ErrorList) {
	ingressList := []networkingv1.Ingress{}
	for _, ing := range storage.Ingresses {
		ingressList = append(ingressList, *ing)
	}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	ir, errs := common.ToIR(ingressList, storage.ServicePorts, c.implementationSpecificOptions)
	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}
	// fix gateway class in gateways
	for i := range ir.Gateways {
		g := ir.Gateways[i]
		g.Spec.GatewayClassName = "kgateway"
		ir.Gateways[i] = g
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(ingressList, storage.ServicePorts, &ir)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	ruleGroups := common.GetRuleGroups(ingressList)

	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			ingress := rule.Ingress
			if pol, ok := c.ing2pol[ingress.Name]; ok {

				key := types.NamespacedName{Namespace: rg.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
				httpRouteContext, ok := ir.HTTPRoutes[key]
				if !ok {
					continue
				}
				if httpRouteContext.ProviderSpecificIR.Kgateway == nil {
					httpRouteContext.ProviderSpecificIR.Kgateway = &intermediate.KgatewayHTTPRouteIR{
						Policies: make(map[string]intermediate.KgatewayPolicy),
					}
				}
				for i, backEndSources := range httpRouteContext.RuleBackendSources {
					for j, backEndSource := range backEndSources {
						if backEndSource.Ingress.Name == ingress.Name {
							key := intermediate.KgatewayPolicyIndex{Rule: i, Backend: j}
							pol.RuleBackendSources = append(pol.RuleBackendSources, key)
						}
					}
				}
				httpRouteContext.ProviderSpecificIR.Kgateway.Policies[ingress.Name] = pol

				ir.HTTPRoutes[key] = httpRouteContext
			}
		}
	}

	return ir, errs
}
