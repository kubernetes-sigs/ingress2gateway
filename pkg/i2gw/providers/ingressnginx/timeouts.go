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

package ingressnginx

import (
	"fmt"
	"strconv"
	"time"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const timeoutMultiplier = 10

func parseIngressNginxTimeout(val string) (time.Duration, error) {
	if val == "" {
		return 0, fmt.Errorf("empty timeout")
	}

	// These annotations are specified as unitless seconds.
	// https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#custom-timeouts
	secs, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("must be an integer number of seconds")
	}
	if secs <= 0 {
		return 0, fmt.Errorf("must be > 0")
	}
	return time.Duration(secs) * time.Second, nil
}

func timeoutFeature(_ []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, pIR *providerir.ProviderIR, eIR *emitterir.EmitterIR) field.ErrorList {
	var errList field.ErrorList

	for key, httpRouteContext := range pIR.HTTPRoutes {
		eHTTPContext := eIR.HTTPRoutes[key]
		eHTTPContext.RequestTimeouts = make(map[int]*gatewayv1.Duration, len(eHTTPContext.Spec.Rules))

		for ruleIdx := range httpRouteContext.HTTPRoute.Spec.Rules {
			sources := httpRouteContext.RuleBackendSources[ruleIdx]
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			var maxTimeout time.Duration
			var any bool
			for _, ann := range []string{ProxyConnectTimeoutAnnotation, ProxySendTimeoutAnnotation, ProxyReadTimeoutAnnotation} {
				if val, ok := ingress.Annotations[ann]; ok && val != "" {
					d, err := parseIngressNginxTimeout(val)
					if err != nil {
						errList = append(errList, field.Invalid(
							field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations").Key(ann),
							val,
							fmt.Sprintf("invalid timeout: %v", err),
						))
						continue
					}
					any = true
					if d > maxTimeout {
						maxTimeout = d
					}
				}
			}

			if !any {
				continue
			}

			maxTimeout *= timeoutMultiplier
			gwDur := gatewayv1.Duration(maxTimeout.String())
			eHTTPContext.RequestTimeouts[ruleIdx] = &gwDur

			notify(notifications.InfoNotification, fmt.Sprintf("parsed ingress-nginx proxy timeouts (x%d) from %s/%s for HTTPRoute %s/%s rule %d (timeouts.request): %s",
				timeoutMultiplier, ingress.Namespace, ingress.Name, key.Namespace, key.Name, ruleIdx, gwDur), &httpRouteContext.HTTPRoute)
			// FIXME, add more docs.
		}

		pIR.HTTPRoutes[key] = httpRouteContext
	}

	if len(errList) > 0 {
		return errList
	}
	return nil
}
