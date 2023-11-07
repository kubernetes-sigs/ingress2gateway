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

import gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

func containsMatches(matches []gatewayv1.HTTPRouteMatch, matchesToCheck []gatewayv1.HTTPRouteMatch) bool {
	if len(matches) != len(matchesToCheck) {
		return false
	}
	for _, matchToCheck := range matchesToCheck {
		var headersFound, methodFound bool
		for _, m := range matches {
			if containsHeaderSet(m.Headers, matchToCheck.Headers) {
				headersFound = true
			}
			if m.Method == nil && matchToCheck.Method != nil || m.Method != nil && matchToCheck.Method == nil {
				continue
			}
			if m.Method == nil || *m.Method == *matchToCheck.Method {
				methodFound = true
			}
			if headersFound && methodFound {
				break
			}
		}
		if !headersFound || !methodFound {
			return false
		}
	}
	return true
}

func containsHeaderSet(matchHeaders []gatewayv1.HTTPHeaderMatch, headerSet []gatewayv1.HTTPHeaderMatch) bool {
	if len(matchHeaders) != len(headerSet) {
		return false
	}
	for _, headerToCheck := range headerSet {
		var found bool
		for _, header := range matchHeaders {
			if headerToCheck.Name == header.Name && headerToCheck.Value == header.Value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
