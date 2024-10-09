package cilium

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// resourcesToIRConverter implements the ToIR function of i2gw.ResourcesToIRConverter interface.
type resourcesToIRConverter struct {
	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

// newResourcesToIRConverter returns a cilium resourcesToIRConverter instance.
func newResourcesToIRConverter() *resourcesToIRConverter {
	return &resourcesToIRConverter{
		featureParsers: []i2gw.FeatureParser{
			forceHTTPSFeature,
		},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			// The list of the implementationSpecific ingress fields options comes here.
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
	ir, errs := common.ToIR(ingressList, c.implementationSpecificOptions)
	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(ingressList, &ir)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return ir, errs
}
