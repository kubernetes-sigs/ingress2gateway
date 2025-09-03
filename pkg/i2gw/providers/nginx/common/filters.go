package common

import (
	"strings"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// CreateResponseHeaderModifier creates a ResponseHeaderModifier filter from comma-separated header names
func CreateResponseHeaderModifier(headersToRemove []string) *gatewayv1.HTTPRouteFilter {
	if len(headersToRemove) == 0 {
		return nil
	}
	return &gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
		ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
			Remove: headersToRemove,
		},
	}
}

// CreateRequestHeaderModifier creates a RequestHeaderModifier filter from header map
func CreateRequestHeaderModifier(headersToSet map[string]string) *gatewayv1.HTTPRouteFilter {
	if len(headersToSet) == 0 {
		return nil
	}
	var headers []gatewayv1.HTTPHeader
	for name, value := range headersToSet {
		if value != "" && !strings.Contains(value, "$") {
			headers = append(headers, gatewayv1.HTTPHeader{
				Name:  gatewayv1.HTTPHeaderName(name),
				Value: value,
			})
		}
	}
	if len(headers) == 0 {
		return nil
	}
	return &gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
			Set: headers,
		},
	}
}

// CreateURLRewriteFilter creates a URLRewrite filter with ReplacePrefixMatch
func CreateURLRewriteFilter(rewritePath string) *gatewayv1.HTTPRouteFilter {
	if rewritePath == "" {
		return nil
	}
	return &gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterURLRewrite,
		URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
			Path: &gatewayv1.HTTPPathModifier{
				Type:               gatewayv1.PrefixMatchHTTPPathModifier,
				ReplacePrefixMatch: &rewritePath,
			},
		},
	}
}