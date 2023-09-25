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

package ingresskong

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func matchingFeature(ingressResources i2gw.InputResources, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingressResources.Ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			// headers matching
			headers := getHeadersAnnotations(rule.Ingress.Annotations)
			key := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: common.NameFromHost(rg.Host)}
			httpRoute, ok := gatewayResources.HTTPRoutes[key]
			if !ok {
				panic("HTTPRoute not exists - this should never happen")
			}
			patchHTTPRouteHeaderMatching(&httpRoute, headers)

			// method matching
			methods, errs := getMethodAnnotation(rule.Ingress.ObjectMeta.Namespace, rule.Ingress.ObjectMeta.Name, rule.Ingress.Annotations)
			if len(errs) != 0 {
				return errs
			}
			patchHTTPRouteMethodMatching(&httpRoute, methods)
		}

	}
	return nil
}

func patchHTTPRouteHeaderMatching(httpRoute *gatewayv1beta1.HTTPRoute, headers map[string]string) {
	for i, rule := range httpRoute.Spec.Rules {
		var oldMatches = make([]gatewayv1beta1.HTTPRouteMatch, 0)
		for _, m := range rule.Matches {
			oldMatches = append(oldMatches, *m.DeepCopy())
		}
		for j := range oldMatches {
			for key, value := range headers {
				if !strings.Contains(value, ",") {
					oldMatches[j].Headers = append(
						oldMatches[j].Headers,
						gatewayv1beta1.HTTPHeaderMatch{
							Name:  gatewayv1beta1.HTTPHeaderName(key),
							Value: value,
						},
					)
				}
			}
			newMatches := make([]gatewayv1beta1.HTTPRouteMatch, 0)
			for key, value := range headers {
				if strings.Contains(value, ",") {
					splitHeaders := strings.Split(value, ",")
					for _, h := range splitHeaders {
						if h == "" {
							continue
						}
						newMatch := oldMatches[j].DeepCopy()
						newMatch.Headers = append(newMatch.Headers,
							gatewayv1beta1.HTTPHeaderMatch{
								Name:  gatewayv1beta1.HTTPHeaderName(key),
								Value: h,
							},
						)
						newMatches = append(newMatches, *newMatch)
					}
				}
			}
			if !hasORedHeaders(headers) {
				httpRoute.Spec.Rules[i].Matches = oldMatches
			} else {
				httpRoute.Spec.Rules[i].Matches = newMatches
			}
		}
	}
}

func hasORedHeaders(headers map[string]string) bool {
	for _, v := range headers {
		if strings.Contains(v, ",") {
			return true
		}
	}
	return false
}

func patchHTTPRouteMethodMatching(httpRoute *gatewayv1beta1.HTTPRoute, methods []gatewayv1beta1.HTTPMethod) {
	for i, rule := range httpRoute.Spec.Rules {
		matches := []gatewayv1beta1.HTTPRouteMatch{}
		for _, match := range rule.Matches {
			for _, method := range methods {
				method := method
				newMatch := match.DeepCopy()
				newMatch.Method = &method
				matches = append(matches, *newMatch)
			}
		}
		httpRoute.Spec.Rules[i].Matches = matches
	}
}

func getHeadersAnnotations(annotations map[string]string) map[string]string {
	headers := make(map[string]string)
	headerAnnotationPrefix := kongAnnotation(headersKey)
	for key, val := range annotations {
		if strings.HasPrefix(key, headerAnnotationPrefix) {
			header := strings.TrimPrefix(key, fmt.Sprintf("%s.", headerAnnotationPrefix))
			if len(header) == 0 || len(val) == 0 {
				continue
			}
			header = strings.TrimPrefix(header, ".")
			headers[header] = val
		}
	}
	return headers
}

func getMethodAnnotation(ingressNamespace, ingressName string, annotations map[string]string) ([]gatewayv1beta1.HTTPMethod, field.ErrorList) {
	fieldPath := field.NewPath(fmt.Sprintf("%s/%s", ingressNamespace, ingressName)).Child("metadata").Child("annotations")
	errs := field.ErrorList{}
	methods := make([]gatewayv1beta1.HTTPMethod, 0)
	mkey := kongAnnotation(methodsKey)
	for key, val := range annotations {
		if key == mkey {
			methodsValues := strings.Split(val, ",")
			for _, v := range methodsValues {
				if err := validateHTTPMethod(gatewayv1beta1.HTTPMethod(v)); err != nil {
					errs = append(errs, field.TypeInvalid(fieldPath, mkey, err.Error()))
					continue
				}
				methods = append(methods, gatewayv1beta1.HTTPMethod(v))
			}
		}
	}
	return methods, errs
}

func validateHTTPMethod(method gatewayv1beta1.HTTPMethod) error {
	if method == gatewayv1beta1.HTTPMethodGet ||
		method == gatewayv1beta1.HTTPMethodHead ||
		method == gatewayv1beta1.HTTPMethodPost ||
		method == gatewayv1beta1.HTTPMethodPut ||
		method == gatewayv1beta1.HTTPMethodDelete ||
		method == gatewayv1beta1.HTTPMethodConnect ||
		method == gatewayv1beta1.HTTPMethodOptions ||
		method == gatewayv1beta1.HTTPMethodTrace ||
		method == gatewayv1beta1.HTTPMethodPatch {
		return nil
	}
	return fmt.Errorf("error %s not supported", method)
}
