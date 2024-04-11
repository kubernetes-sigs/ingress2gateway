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
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// headerMatchingFeature parses the Kong Ingress Controller headers annotations and convert them
// into HTTPRoutes Header Matching configurations.
//
// Kong ingress Controller allows defining headers matching via the following annotation:
// konghq.com/headers.name1: "value1,value2"
// konghq.com/headers.name2: "value3"
//
// All the values defined for each annotation name, and separated by comma, MUST be ORed.
// All the annotation names MUST be ANDed, with the respective values.
func headerMatchingFeature(ingressResources i2gw.InputResources, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingressResources.Ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			headerskeys, headersValues := parseHeadersAnnotations(rule.Ingress.Annotations)
			key := types.NamespacedName{Namespace: rule.Ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRoute, ok := gatewayResources.HTTPRoutes[key]
			if !ok {
				return field.ErrorList{field.InternalError(nil, fmt.Errorf("HTTPRoute does not exist - this should never happen"))}
			}

			patchHTTPRouteHeaderMatching(&httpRoute, headerskeys, headersValues)
		}

	}
	return nil
}

func patchHTTPRouteHeaderMatching(httpRoute *gatewayv1.HTTPRoute, headerNames []string, headerValues [][]string) {
	for i := range httpRoute.Spec.Rules {
		newMatches := []gatewayv1.HTTPRouteMatch{}
		for _, match := range httpRoute.Spec.Rules[i].Matches {
			headersIndexes := make([]int, len(headerNames))
			// the current match is duplicated, as each ORed header value requires a new match.
			for j := 0; j < getNumberOfMatches(headerNames, headerValues); j++ {
				newMatches = append(newMatches, match)
			}
			// iterate over the matches and populate them with the proper headers.
			for j := range newMatches {
				newMatches[j].Headers = make([]gatewayv1.HTTPHeaderMatch, len(headerNames))
				for k, name := range headerNames {
					newMatches[j].Headers[k].Name = gatewayv1.HTTPHeaderName(name)
					index := headersIndexes[k]
					value := headerValues[k][index]
					newMatches[j].Headers[k].Value = value
					index++
					if index >= len(headerValues[k]) {
						index = 0
					}
					headersIndexes[k] = index
				}
			}
		}
		httpRoute.Spec.Rules[i].Matches = newMatches
	}
}

// parseHeadersAnnotations returns two different datasets:
//   - headersNames is a slice with all the headers names
//   - headersValues is a slice of slices where the first index corresponds to the headersNames[*] value
func parseHeadersAnnotations(annotations map[string]string) (headersNames []string, headersValues [][]string) {
	headerAnnotationPrefix := kongAnnotation(headersKey)
	headers := make(map[string][]string)
	for key, val := range annotations {
		if strings.HasPrefix(key, headerAnnotationPrefix) {
			headerName := strings.TrimPrefix(key, fmt.Sprintf("%s.", headerAnnotationPrefix))
			if len(headerName) == 0 || len(val) == 0 {
				continue
			}
			splitVals := strings.Split(val, ",")
			var n int
			for _, val := range splitVals {
				if val != "" {
					n++
				}
			}
			headers[headerName] = make([]string, n)
			var i int
			for _, headerVal := range splitVals {
				if headerVal != "" {
					headers[headerName][i] = headerVal
					i++
				}
			}
		}
	}
	headersNames = make([]string, len(headers))
	headersValues = make([][]string, len(headers))
	var i int
	for key, vals := range headers {
		headersNames[i] = key
		headersValues[i] = make([]string, len(headers[key]))
		copy(headersValues[i], vals)
		i++
	}
	return
}

func getNumberOfMatches(headerKeys []string, headerValues [][]string) (n int) {
	n = 1
	for i := range headerKeys {
		n = n * len(headerValues[i])
	}
	return
}
