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

package kong

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// methodMatchingFeature parses the Kong Ingress Controller methods annotations and convert them
// into HTTPRoutes Method Matching configurations.
//
// Kong ingress Controller allows defining method matching via the following annotation:
// konghq.com/methods: "GET,POST"
//
// All the values defined and separated by comma, MUST be ORed.
func methodMatchingFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			key := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRouteContext, ok := ir.HTTPRoutes[key]
			if !ok {
				return field.ErrorList{field.InternalError(nil, fmt.Errorf("HTTPRoute does not exist - this should never happen"))}
			}
			methods, errs := parseMethodsAnnotation(rule.Ingress.ObjectMeta.Namespace, rule.Ingress.ObjectMeta.Name, rule.Ingress.Annotations)
			if len(errs) != 0 {
				return errs
			}
			patchHTTPRouteMethodMatching(notify, &httpRouteContext.HTTPRoute, methods)
		}
	}
	return nil
}

func patchHTTPRouteMethodMatching(notify notifications.NotifyFunc, httpRoute *gatewayv1.HTTPRoute, methods []gatewayv1.HTTPMethod) {
	for i, rule := range httpRoute.Spec.Rules {
		matches := []gatewayv1.HTTPRouteMatch{}
		for _, match := range rule.Matches {
			for _, method := range methods {
				method := method
				newMatch := match.DeepCopy()
				newMatch.Method = &method
				matches = append(matches, *newMatch)
			}
		}
		if len(matches) > 0 {
			httpRoute.Spec.Rules[i].Matches = matches
			notify(notifications.InfoNotification, fmt.Sprintf("parsed \"%v\" annotation of ingress and patched %v fields", kongAnnotation(methodsKey), field.NewPath("httproute", "spec", "rules").Key("").Child("matches").Key("").Child("method")), httpRoute)
		}
	}
}

func parseMethodsAnnotation(ingressNamespace, ingressName string, annotations map[string]string) ([]gatewayv1.HTTPMethod, field.ErrorList) {
	fieldPath := field.NewPath(fmt.Sprintf("%s/%s", ingressNamespace, ingressName)).Child("metadata").Child("annotations").Child(kongAnnotation(methodsKey))
	errs := field.ErrorList{}
	methods := make([]gatewayv1.HTTPMethod, 0)
	mkey := kongAnnotation(methodsKey)
	for key, val := range annotations {
		if key == mkey {
			methodsValues := strings.Split(val, ",")
			for _, v := range methodsValues {
				if err := validateHTTPMethod(gatewayv1.HTTPMethod(v)); err != nil {
					errs = append(errs, field.Invalid(fieldPath, v, err.Error()))
					continue
				}
				methods = append(methods, gatewayv1.HTTPMethod(v))
			}
		}
	}
	return methods, errs
}

func validateHTTPMethod(method gatewayv1.HTTPMethod) error {
	if method == gatewayv1.HTTPMethodGet ||
		method == gatewayv1.HTTPMethodHead ||
		method == gatewayv1.HTTPMethodPost ||
		method == gatewayv1.HTTPMethodPut ||
		method == gatewayv1.HTTPMethodDelete ||
		method == gatewayv1.HTTPMethodConnect ||
		method == gatewayv1.HTTPMethodOptions ||
		method == gatewayv1.HTTPMethodTrace ||
		method == gatewayv1.HTTPMethodPatch {
		return nil
	}
	return errors.New("method not supported")
}
