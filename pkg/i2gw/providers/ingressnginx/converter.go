/*
Copyright 2023 The Kubernetes Authors.

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

package ingressnginx

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// resourcesToIRConverter implements the ToIR function of i2gw.ResourcesToIRConverter interface.
type resourcesToIRConverter struct {
	featureParsers []i2gw.FeatureParser
	notify         notifications.NotifyFunc
}

// newResourcesToIRConverter returns an ingress-nginx resourcesToIRConverter instance.
func newResourcesToIRConverter(notify notifications.NotifyFunc) *resourcesToIRConverter {
	return &resourcesToIRConverter{
		featureParsers: []i2gw.FeatureParser{
			canaryFeature,
			createBackendTLSPolicies,
			redirectFeature,
			headerModifierFeature,
			regexFeature,
			backendTLSFeature,
			sessionAffinityFeature,
			sslPassthroughFeature,
			appRootFeature,
		},
		notify: notify,
	}
}

func (c *resourcesToIRConverter) convert(notify notifications.NotifyFunc, storage *storage) (providerir.ProviderIR, field.ErrorList) {

	// TODO(liorliberman) temporary until we decide to change ToIR and featureParsers to get a map of [types.NamespacedName]*networkingv1.Ingress instead of a list
	ingressList := storage.Ingresses.List()

	// Filter Ingresses: Split into HTTP and GRPC
	var httpIngresses []networkingv1.Ingress
	var grpcIngresses []networkingv1.Ingress

	for _, ing := range ingressList {
		if val, ok := ing.Annotations[BackendProtocolAnnotation]; ok {
			switch strings.ToUpper(val) {
			case "GRPC", "GRPCS":
				grpcIngresses = append(grpcIngresses, ing)
			case "HTTP", "HTTPS", "AUTO_HTTP":
				httpIngresses = append(httpIngresses, ing)
			default:
				// Should cover FCGI and unknown
				notify(notifications.WarningNotification, fmt.Sprintf("%s backend-protocol is not supported in Gateway API conversion for ingress %s/%s", val, ing.Namespace, ing.Name), nil)
			}
		} else {
			httpIngresses = append(httpIngresses, ing)
		}
	}

	// Warn that gRPC support is not fully fleshed out and some untranslated
	// behavior may not be reported.
	if len(grpcIngresses) > 0 {
		notify(notifications.WarningNotification, "GRPC support is not fully implemented. Some Ingress-NGINX GRPC behaviors may not be correctly translated, and untranslated behavior may not be notified.")
	}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	pIR, errs := common.ToIR(httpIngresses, grpcIngresses, storage.ServicePorts, i2gw.ProviderImplementationSpecificOptions{
		ToImplementationSpecificHTTPPathTypeMatch: implementationSpecificPathMatch,
	})

	// Warn about hosts that lack TLS certificates. Ingress NGINX serves TLS
	// for all hosts using a self-signed certificate when no explicit cert is
	// configured. We do not translate this behavior.
	for _, gwCtx := range pIR.Gateways {
		httpsHosts := map[string]struct{}{}
		var httpHosts []string
		for _, listener := range gwCtx.Gateway.Spec.Listeners {
			if listener.Hostname == nil {
				continue
			}
			host := string(*listener.Hostname)
			switch listener.Port {
			case 443:
				httpsHosts[host] = struct{}{}
			case 80:
				httpHosts = append(httpHosts, host)
			}
		}
		for _, host := range httpHosts {
			if _, ok := httpsHosts[host]; !ok {
				c.notify(notifications.WarningNotification, fmt.Sprintf(
					"Ingress NGINX serves TLS traffic for host %q with a self-signed certificate. This behavior will not be translated and the host will not be accessible via HTTPS.",
					host))
			}
		}
	}

	for _, ingress := range ingressList {
		for annotation := range ingress.Annotations {
			if _, ok := parsedAnnotations[annotation]; !ok && strings.HasPrefix(annotation, ingressNGINXAnnotationsPrefix) {
				c.notify(notifications.WarningNotification, fmt.Sprintf("Unsupported annotation %v", annotation), &ingress)
			}
		}
	}

	if len(errs) > 0 {
		return providerir.ProviderIR{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(c.notify, ingressList, storage.ServicePorts, &pIR)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return pIR, errs
}

func implementationSpecificPathMatch(path *gatewayv1.HTTPPathMatch) {
	// Nginx Ingress Controller treats ImplementationSpecific as Prefix by default,
	// unless regex characters are present (handled by regexFeature).
	// We safely default to Prefix here to pass the common.ToIR check.
	t := gatewayv1.PathMatchPathPrefix
	path.Type = &t
}
