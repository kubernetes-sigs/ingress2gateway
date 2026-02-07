/*
Copyright 2025 The Kubernetes Authors.

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

package annotations

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// GRPCServicesFeature processes nginx.org/grpc-services annotation
func GRPCServicesFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			if grpcServices, exists := rule.Ingress.Annotations[nginxGRPCServicesAnnotation]; exists && grpcServices != "" {
				errs = append(errs, processGRPCServicesAnnotation(rule.Ingress, grpcServices, ir)...)
			}
		}
	}

	return errs
}

// processGRPCServicesAnnotation handles gRPC backend services
//
//nolint:unparam // ErrorList return type maintained for consistency
func processGRPCServicesAnnotation(ingress networkingv1.Ingress, grpcServices string, ir *providerir.ProviderIR) field.ErrorList {
	var errs field.ErrorList //nolint:unparam // ErrorList return type maintained for consistency

	// Parse comma-separated service names that should use gRPC
	services := splitAndTrimCommaList(grpcServices)
	grpcServiceSet := make(map[string]struct{})
	for _, service := range services {
		grpcServiceSet[service] = struct{}{}
	}

	// Initialize GRPCRoutes map if needed
	if ir.GRPCRoutes == nil {
		ir.GRPCRoutes = make(map[types.NamespacedName]providerir.GRPCRouteContext)
	}

	// Mark services as gRPC in provider-specific IR
	if ir.Services == nil {
		ir.Services = make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR)
	}

	// Process each ingress rule that uses gRPC services
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		routeName := common.RouteName(ingress.Name, rule.Host)
		routeKey := types.NamespacedName{
			Namespace: ingress.Namespace,
			Name:      routeName,
		}

		var grpcRouteRules []gatewayv1.GRPCRouteRule
		var remainingHTTPRules []gatewayv1.HTTPRouteRule

		// Get existing HTTPRoute to copy filters and check for rules
		httpRouteContext, httpRouteExists := ir.HTTPRoutes[routeKey]

		// Separate gRPC paths from non-gRPC paths
		for _, path := range rule.HTTP.Paths {
			serviceName := path.Backend.Service.Name
			if _, exists := grpcServiceSet[serviceName]; exists {
				// This path uses a gRPC service - create GRPCRoute rule
				grpcMatch := gatewayv1.GRPCRouteMatch{}

				// Convert HTTP path to gRPC service/method match
				if path.Path != "" {
					service, method := common.ParseGRPCServiceMethod(path.Path)
					if service != "" {
						grpcMatch.Method = &gatewayv1.GRPCMethodMatch{
							Service: &service,
						}
						if method != "" {
							grpcMatch.Method.Method = &method
						}
					}
				}

				// Create backend reference
				var port *gatewayv1.PortNumber
				if path.Backend.Service.Port.Number != 0 {
					portNum := gatewayv1.PortNumber(path.Backend.Service.Port.Number)
					port = &portNum
				}

				backendRef := gatewayv1.GRPCBackendRef{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(serviceName),
							Port: port,
						},
					},
				}

				// Copy filters from HTTPRoute to GRPCRoute rule
				var grpcFilters []gatewayv1.GRPCRouteFilter
				if httpRouteExists {
					// Find the corresponding HTTP rule for this path to copy its filters
					grpcFilters = findAndConvertFiltersForGRPCPath(httpRouteContext.HTTPRoute.Spec.Rules, path.Path)
				}

				grpcRule := gatewayv1.GRPCRouteRule{
					Matches:     []gatewayv1.GRPCRouteMatch{grpcMatch},
					Filters:     grpcFilters,
					BackendRefs: []gatewayv1.GRPCBackendRef{backendRef},
				}

				grpcRouteRules = append(grpcRouteRules, grpcRule)
			}
		}

		// Create GRPCRoute if we have any gRPC rules
		if len(grpcRouteRules) > 0 {
			var hostnames []gatewayv1.Hostname
			if rule.Host != "" {
				hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(rule.Host)}
			}

			grpcRoute := gatewayv1.GRPCRoute{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gatewayv1.GroupVersion.String(),
					Kind:       GRPCRouteKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      routeName,
					Namespace: ingress.Namespace,
				},
				Spec: gatewayv1.GRPCRouteSpec{
					CommonRouteSpec: gatewayv1.CommonRouteSpec{
						ParentRefs: []gatewayv1.ParentReference{
							{
								Name: func() gatewayv1.ObjectName {
									if ingress.Spec.IngressClassName != nil {
										return gatewayv1.ObjectName(*ingress.Spec.IngressClassName)
									}
									return NginxIngressClass
								}(),
							},
						},
					},
					Hostnames: hostnames,
					Rules:     grpcRouteRules,
				},
			}

			ir.GRPCRoutes[routeKey] = providerir.GRPCRouteContext{
				GRPCRoute: grpcRoute,
			}

			// Remove HTTP rules that correspond to gRPC services from the HTTPRoute
			if httpRouteExists {
				remainingHTTPRules = common.RemoveGRPCRulesFromHTTPRoute(&httpRouteContext.HTTPRoute, grpcServiceSet)

				// If no rules remain, remove the entire HTTPRoute
				if len(remainingHTTPRules) == 0 {
					delete(ir.HTTPRoutes, routeKey)
				} else {
					// Update HTTPRoute with remaining rules
					httpRouteContext.HTTPRoute.Spec.Rules = remainingHTTPRules
					ir.HTTPRoutes[routeKey] = httpRouteContext
				}
			}
		}
	}

	return errs
}

// findAndConvertFiltersForGRPCPath finds the HTTP rule that matches the given path and converts its filters to gRPC filters
func findAndConvertFiltersForGRPCPath(httpRules []gatewayv1.HTTPRouteRule, grpcPath string) []gatewayv1.GRPCRouteFilter {
	// Find the HTTP rule that contains this path
	for _, httpRule := range httpRules {
		for _, match := range httpRule.Matches {
			if match.Path != nil && match.Path.Value != nil && *match.Path.Value == grpcPath {
				// Found the matching rule, convert its filters
				conversionResult := common.ConvertHTTPFiltersToGRPCFilters(httpRule.Filters)

				// Handle notifications for unsupported filters
				for _, unsupportedType := range conversionResult.UnsupportedTypes {
					switch unsupportedType {
					case gatewayv1.HTTPRouteFilterRequestHeaderModifier:
						// This should never happen as it's a supported filter, but added for exhaustiveness
						notify(notifications.WarningNotification, "RequestHeaderModifier should be supported for gRPC")
					case gatewayv1.HTTPRouteFilterResponseHeaderModifier:
						// This should never happen as it's a supported filter, but added for exhaustiveness
						notify(notifications.WarningNotification, "ResponseHeaderModifier should be supported for gRPC")
					case gatewayv1.HTTPRouteFilterRequestRedirect:
						notify(notifications.WarningNotification, "RequestRedirect is not applicable to gRPC")
					case gatewayv1.HTTPRouteFilterURLRewrite:
						notify(notifications.WarningNotification, "URLRewrite is not applicable to gRPC")
					case gatewayv1.HTTPRouteFilterRequestMirror:
						notify(notifications.WarningNotification, "RequestMirror is not applicable to gRPC")
					case gatewayv1.HTTPRouteFilterExtensionRef:
						notify(notifications.WarningNotification, "ExtensionRef filters are not converted to gRPC equivalents")
					case gatewayv1.HTTPRouteFilterCORS:
						notify(notifications.WarningNotification, "CORS is not applicable to gRPC")
					case gatewayv1.HTTPRouteFilterExternalAuth:
						notify(notifications.WarningNotification, "ExternalAuth is not applicable to gRPC")
					default:
						notify(notifications.WarningNotification, "Unknown HTTPRouteFilter type: "+string(unsupportedType))
					}
				}
				return conversionResult.GRPCFilters
			}
		}
	}
	return nil
}
