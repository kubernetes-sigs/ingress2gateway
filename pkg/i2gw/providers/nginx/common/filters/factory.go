package filters

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Header defines an HTTP Header.
type Header struct {
	Name  string
	Value string
}

// FilterType represents the type of filter to create
type FilterType string

const (
	RequestHeaderModifierFilter  FilterType = "RequestHeaderModifier"
	ResponseHeaderModifierFilter FilterType = "ResponseHeaderModifier"
	URLRewriteFilter             FilterType = "URLRewrite"
	RequestRedirectFilter        FilterType = "RequestRedirect"
)

// HeaderModifierOptions contains options for header modification filters
type HeaderModifierOptions struct {
	SetHeaders    []Header
	RemoveHeaders []string
}

// URLRewriteOptions contains options for URL rewrite filters
type URLRewriteOptions struct {
	// Path is the path to rewrite
	Path string
}

// RequestRedirectOptions contains options for request redirect filters
type RequestRedirectOptions struct {
	// StatusCode represents the HTTP status code for the redirect
	StatusCode int
	// Hostname is the hostname to redirect to
	Hostname string
	// Path is the path to redirect to
	Path string
	// Scheme is the scheme to redirect to (http/https)
	Scheme string
	// Port is the port to redirect to
	Port *int32
}

// FilterOptions contains all filter configuration options
type FilterOptions struct {
	HeaderModifier        *HeaderModifierOptions
	URLRewrite            *URLRewriteOptions
	RequestRedirect       *RequestRedirectOptions
	NotificationCollector common.NotificationCollector
	SourceObject          client.Object
}

// NewHTTPRouteFilter creates a Gateway API HTTPRouteFilter based on the filter type and options
func NewHTTPRouteFilter(filterType FilterType, opts FilterOptions) *gatewayv1.HTTPRouteFilter {
	switch filterType {
	case RequestHeaderModifierFilter:
		return createRequestHeaderModifierFilter(opts.HeaderModifier, opts.NotificationCollector, opts.SourceObject)
	case ResponseHeaderModifierFilter:
		return createResponseHeaderModifierFilter(opts.HeaderModifier, opts.NotificationCollector, opts.SourceObject)
	case URLRewriteFilter:
		return createURLRewriteFilter(opts.URLRewrite, opts.NotificationCollector, opts.SourceObject)
	case RequestRedirectFilter:
		return createRequestRedirectFilter(opts.RequestRedirect, opts.NotificationCollector, opts.SourceObject)
	default:
		return nil
	}
}

// createRequestHeaderModifierFilter creates a RequestHeaderModifier filter
func createRequestHeaderModifierFilter(headerOpts *HeaderModifierOptions, notificationCollector common.NotificationCollector, sourceObj client.Object) *gatewayv1.HTTPRouteFilter {
	if headerOpts == nil {
		return nil
	}

	modifier := &gatewayv1.HTTPHeaderFilter{}
	skippedHeaders := 0

	if len(headerOpts.SetHeaders) > 0 {
		// Handle header structs (for annotations compatibility)
		var headers []gatewayv1.HTTPHeader
		for _, header := range headerOpts.SetHeaders {
			// Check for NGINX variables
			if strings.Contains(header.Value, "$") {
				skippedHeaders++
				if notificationCollector != nil {
					notificationCollector.AddWarning(
						fmt.Sprintf("Request header '%s' with NGINX variable value skipped - Gateway API doesn't support dynamic values", header.Name),
						sourceObj)
				}
				continue
			}
			headers = append(headers, gatewayv1.HTTPHeader{
				Name:  gatewayv1.HTTPHeaderName(header.Name),
				Value: header.Value,
			})
		}
		modifier.Set = headers

		if notificationCollector != nil && len(headers) > 0 {
			notificationCollector.AddInfo(
				fmt.Sprintf("Request header modifications converted (%d headers)", len(headers)),
				sourceObj)
		}
		if notificationCollector != nil && skippedHeaders > 0 {
			notificationCollector.AddWarning(
				fmt.Sprintf("%d request headers with NGINX variables skipped - Gateway API doesn't support dynamic values", skippedHeaders),
				sourceObj)
		}
	}

	// Handle header removal
	if len(headerOpts.RemoveHeaders) > 0 {
		modifier.Remove = headerOpts.RemoveHeaders
	}

	// Return nil if no modifications are defined
	if len(modifier.Set) == 0 && len(modifier.Remove) == 0 {
		return nil
	}

	return &gatewayv1.HTTPRouteFilter{
		Type:                  gatewayv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: modifier,
	}
}

// createResponseHeaderModifierFilter creates a ResponseHeaderModifier filter
func createResponseHeaderModifierFilter(headerOpts *HeaderModifierOptions, notificationCollector common.NotificationCollector, sourceObj client.Object) *gatewayv1.HTTPRouteFilter {
	if headerOpts == nil {
		return nil
	}

	modifier := &gatewayv1.HTTPHeaderFilter{}
	skippedHeaders := 0

	if len(headerOpts.SetHeaders) > 0 {
		// Handle header structs (for annotations compatibility)
		var headers []gatewayv1.HTTPHeader
		for _, header := range headerOpts.SetHeaders {
			// Check for NGINX variables
			if strings.Contains(header.Value, "$") {
				skippedHeaders++
				if notificationCollector != nil {
					notificationCollector.AddWarning(
						fmt.Sprintf("Response header '%s' with NGINX variable value skipped - Gateway API doesn't support dynamic values", header.Name),
						sourceObj)
				}
				continue
			}
			headers = append(headers, gatewayv1.HTTPHeader{
				Name:  gatewayv1.HTTPHeaderName(header.Name),
				Value: header.Value,
			})
		}
		modifier.Set = headers

		if notificationCollector != nil && len(headers) > 0 {
			notificationCollector.AddInfo(
				fmt.Sprintf("Response header modifications converted (%d headers)", len(headers)),
				sourceObj)
		}
		if notificationCollector != nil && skippedHeaders > 0 {
			notificationCollector.AddWarning(
				fmt.Sprintf("%d response headers with NGINX variables skipped - Gateway API doesn't support dynamic values", skippedHeaders),
				sourceObj)
		}
	}

	// Handle header removal
	if len(headerOpts.RemoveHeaders) > 0 {
		modifier.Remove = headerOpts.RemoveHeaders
	}

	// Return nil if no modifications are defined
	if len(modifier.Set) == 0 && len(modifier.Remove) == 0 {
		return nil
	}

	return &gatewayv1.HTTPRouteFilter{
		Type:                   gatewayv1.HTTPRouteFilterResponseHeaderModifier,
		ResponseHeaderModifier: modifier,
	}
}

// createURLRewriteFilter creates a URLRewrite filter
func createURLRewriteFilter(
	rewriteOpts *URLRewriteOptions,
	notificationCollector common.NotificationCollector,
	sourceObj client.Object,
) *gatewayv1.HTTPRouteFilter {
	if rewriteOpts == nil || rewriteOpts.Path == "" {
		return nil
	}

	// Always use PrefixMatchHTTPPathModifier
	modifier := gatewayv1.HTTPPathModifier{
		Type:               gatewayv1.PrefixMatchHTTPPathModifier,
		ReplacePrefixMatch: &rewriteOpts.Path,
	}

	// Notify that we applied a prefix rewrite
	if notificationCollector != nil {
		notificationCollector.AddInfo(
			fmt.Sprintf("Path rewrite converted to prefix replacement: %s", rewriteOpts.Path),
			sourceObj,
		)
	}

	return &gatewayv1.HTTPRouteFilter{
		Type:       gatewayv1.HTTPRouteFilterURLRewrite,
		URLRewrite: &gatewayv1.HTTPURLRewriteFilter{Path: &modifier},
	}
}

// createRequestRedirectFilter creates a RequestRedirect filter
func createRequestRedirectFilter(redirectOpts *RequestRedirectOptions, notificationCollector common.NotificationCollector, sourceObj client.Object) *gatewayv1.HTTPRouteFilter {
	if redirectOpts == nil {
		return nil
	}

	redirect := &gatewayv1.HTTPRequestRedirectFilter{}

	// Set status code (default to 302 if not specified)
	if redirectOpts.StatusCode > 0 {
		redirect.StatusCode = &redirectOpts.StatusCode
	} else {
		statusCode := 302
		redirect.StatusCode = &statusCode
	}

	// Set hostname if specified
	if redirectOpts.Hostname != "" {
		redirect.Hostname = (*gatewayv1.PreciseHostname)(&redirectOpts.Hostname)
	}

	// Set path if specified
	if redirectOpts.Path != "" {
		strPtr := func(s string) *string { return &s }
		redirect.Path = &gatewayv1.HTTPPathModifier{
			Type:            gatewayv1.FullPathHTTPPathModifier,
			ReplaceFullPath: strPtr(redirectOpts.Path),
		}
	}

	// Set scheme if specified
	if redirectOpts.Scheme != "" {
		redirect.Scheme = &redirectOpts.Scheme
	}

	// Set port if specified
	if redirectOpts.Port != nil {
		redirect.Port = (*gatewayv1.PortNumber)(redirectOpts.Port)
	}

	if notificationCollector != nil {
		message := fmt.Sprintf("Request redirect converted (status: %d)", *redirect.StatusCode)
		if redirectOpts.Hostname != "" {
			message += fmt.Sprintf(", hostname: %s", redirectOpts.Hostname)
		}
		if redirectOpts.Path != "" {
			message += fmt.Sprintf(", path: %s", redirectOpts.Path)
		}
		notificationCollector.AddInfo(message, sourceObj)
	}

	return &gatewayv1.HTTPRouteFilter{
		Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: redirect,
	}
}
