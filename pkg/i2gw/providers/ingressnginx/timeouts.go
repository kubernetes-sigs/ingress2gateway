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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

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

// applyTimeoutsToEmitterIR is a temporary bridge until timeout parsing is integrated
// into the generic feature parsing flow.
func (p *Provider) applyTimeoutsToEmitterIR(pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) {

	for key, httpRouteContext := range pIR.HTTPRoutes {
		eHTTPContext, ok := eIR.HTTPRoutes[key]
		if !ok {
			continue
		}
		if eHTTPContext.TCPTimeoutsByRuleIdx == nil {
			eHTTPContext.TCPTimeoutsByRuleIdx = make(map[int]*emitterir.TCPTimeouts, len(eHTTPContext.Spec.Rules))
		}

		for ruleIdx := range httpRouteContext.HTTPRoute.Spec.Rules {
			if ruleIdx >= len(httpRouteContext.RuleBackendSources) {
				continue
			}
			sources := httpRouteContext.RuleBackendSources[ruleIdx]
			ingress := getNonCanaryIngress(sources)
			if ingress == nil {
				continue
			}

			connect := p.parseIngressNginxTimeoutAnnotation(ingress, ProxyConnectTimeoutAnnotation)
			read := p.parseIngressNginxTimeoutAnnotation(ingress, ProxyReadTimeoutAnnotation)
			write := p.parseIngressNginxTimeoutAnnotation(ingress, ProxySendTimeoutAnnotation)
			if connect == nil && read == nil && write == nil {
				continue
			}

			eHTTPContext.TCPTimeoutsByRuleIdx[ruleIdx] = &emitterir.TCPTimeouts{
				Connect: connect,
				Read:    read,
				Write:   write,
			}

			p.notify(
				notifications.WarningNotification,
				"ingress-nginx only supports TCP-level timeouts; i2gw has made a best-effort translation to Gateway API timeouts.request."+
					" Please verify that this meets your needs. See documentation: https://gateway-api.sigs.k8s.io/guides/http-timeouts/",
			)
		}

		eIR.HTTPRoutes[key] = eHTTPContext
	}
}

func (p *Provider) parseIngressNginxTimeoutAnnotation(ingress *networkingv1.Ingress, annotation string) *gatewayv1.Duration {
	val, ok := ingress.Annotations[annotation]
	if !ok || val == "" {
		return nil
	}
	d, err := parseIngressNginxTimeout(val)
	if err != nil {
		p.notify(notifications.WarningNotification, fmt.Sprintf("Invalid timeout annotation %s=%q: %v, skipping timeout",
			annotation, val, err), ingress)
		return nil
	}
	gwDur := gatewayv1.Duration(d.String())
	return &gwDur
}
