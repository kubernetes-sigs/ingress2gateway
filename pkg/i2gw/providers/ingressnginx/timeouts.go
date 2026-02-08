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
	"strings"
	"time"

	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
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

// applyTimeoutsToEmitterIR is a temporary bridge until timeout parsing is integrated
// into the generic feature parsing flow.
func applyTimeoutsToEmitterIR(pIR providerir.ProviderIR, eIR *emitterir.EmitterIR) field.ErrorList {
	var errList field.ErrorList

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

			connect, _ := parseIngressNginxTimeoutAnnotation(ingress, ProxyConnectTimeoutAnnotation, &errList)
			read, _ := parseIngressNginxTimeoutAnnotation(ingress, ProxyReadTimeoutAnnotation, &errList)
			write, _ := parseIngressNginxTimeoutAnnotation(ingress, ProxySendTimeoutAnnotation, &errList)
			if connect == nil && read == nil && write == nil {
				continue
			}

			eHTTPContext.TCPTimeoutsByRuleIdx[ruleIdx] = &emitterir.TCPTimeouts{
				Connect: connect,
				Read:    read,
				Write:   write,
			}

			notify(notifications.InfoNotification, fmt.Sprintf("parsed ingress-nginx proxy timeouts (x%d) from %s/%s for HTTPRoute %s/%s rule %d (timeouts.request): %s",
				timeoutMultiplier, ingress.Namespace, ingress.Name, key.Namespace, key.Name, ruleIdx, formatTCPTimeouts(connect, read, write)), &httpRouteContext.HTTPRoute)
			notify(
				notifications.WarningNotification,
				"ingress-nginx only supports TCP-level timeouts; i2gw has made a best-effort translation to Gateway API timeouts.request."+
					" Please verify that this meets your needs. See documentation: https://gateway-api.sigs.k8s.io/guides/http-timeouts/",
			)
		}

		eIR.HTTPRoutes[key] = eHTTPContext
	}

	if len(errList) > 0 {
		return errList
	}
	return nil
}

func parseIngressNginxTimeoutAnnotation(ingress *networkingv1.Ingress, annotation string, errList *field.ErrorList) (*gatewayv1.Duration, error) {
	val, ok := ingress.Annotations[annotation]
	if !ok || val == "" {
		return nil, nil
	}
	d, err := parseIngressNginxTimeout(val)
	if err != nil {
		*errList = append(*errList, field.Invalid(
			field.NewPath("ingress", ingress.Namespace, ingress.Name, "metadata", "annotations").Key(annotation),
			val,
			fmt.Sprintf("invalid timeout: %v", err),
		))
		return nil, err
	}
	gwDur := gatewayv1.Duration(d.String())
	return &gwDur, nil
}

func formatTCPTimeouts(connect, read, write *gatewayv1.Duration) string {
	parts := []string{}
	if connect != nil {
		parts = append(parts, fmt.Sprintf("connect=%s", *connect))
	}
	if read != nil {
		parts = append(parts, fmt.Sprintf("read=%s", *read))
	}
	if write != nil {
		parts = append(parts, fmt.Sprintf("write=%s", *write))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
