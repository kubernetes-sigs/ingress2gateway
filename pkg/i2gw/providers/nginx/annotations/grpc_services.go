/*
Copyright 2024 The Kubernetes Authors.

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
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// GRPCServicesFeature processes nginx.org/grpc-services annotation
func GRPCServicesFeature(ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
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

// parseGRPCServiceMethod parses gRPC service and method from HTTP path
func parseGRPCServiceMethod(path string) (service, method string) {
	path = strings.TrimPrefix(path, "/")

	parts := strings.SplitN(path, "/", 2)
	if len(parts) >= 1 && parts[0] != "" {
		service = parts[0]
	}
	if len(parts) >= 2 && parts[1] != "" {
		method = parts[1]
	}

	return service, method
}

// processGRPCServicesAnnotation handles gRPC backend services
//
//nolint:unparam // ErrorList return type maintained for consistency
func processGRPCServicesAnnotation(ingress networkingv1.Ingress, grpcServices string, ir *intermediate.IR) field.ErrorList {
	var errs field.ErrorList //nolint:unparam // ErrorList return type maintained for consistency

	// Parse comma-separated service names that should use gRPC
	services := strings.Split(grpcServices, ",")
	grpcServiceSet := make(map[string]struct{})
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service != "" {
			grpcServiceSet[service] = struct{}{}
		}
	}

	// Initialize GRPCRoutes map if needed
	if ir.GRPCRoutes == nil {
		ir.GRPCRoutes = make(map[types.NamespacedName]gatewayv1.GRPCRoute)
	}

	// Mark services as gRPC in provider-specific IR
	if ir.Services == nil {
		ir.Services = make(map[types.NamespacedName]intermediate.ProviderSpecificServiceIR)
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
					service, method := parseGRPCServiceMethod(path.Path)
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
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "ingress2gateway",
						"ingress2gateway.io/source":    "nginx-grpc-services",
					},
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

			ir.GRPCRoutes[routeKey] = grpcRoute

			// Remove HTTP rules that correspond to gRPC services from the HTTPRoute
			if httpRouteExists {
				remainingHTTPRules = removeGRPCRulesFromHTTPRoute(&httpRouteContext.HTTPRoute, grpcServiceSet)

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
				return convertHTTPFiltersToGRPCFilters(httpRule.Filters)
			}
		}
	}
	return nil
}

// convertHTTPFiltersToGRPCFilters converts a list of HTTPRoute filters to GRPCRoute filters
func convertHTTPFiltersToGRPCFilters(httpFilters []gatewayv1.HTTPRouteFilter) []gatewayv1.GRPCRouteFilter {
	var grpcFilters []gatewayv1.GRPCRouteFilter

	for _, httpFilter := range httpFilters {
		var grpcFilter gatewayv1.GRPCRouteFilter

		switch httpFilter.Type {
		case gatewayv1.HTTPRouteFilterRequestHeaderModifier:
			if httpFilter.RequestHeaderModifier != nil {
				grpcFilter = gatewayv1.GRPCRouteFilter{
					Type: gatewayv1.GRPCRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Set:    httpFilter.RequestHeaderModifier.Set,
						Add:    httpFilter.RequestHeaderModifier.Add,
						Remove: httpFilter.RequestHeaderModifier.Remove,
					},
				}
				grpcFilters = append(grpcFilters, grpcFilter)
			}

		case gatewayv1.HTTPRouteFilterResponseHeaderModifier:
			if httpFilter.ResponseHeaderModifier != nil {
				grpcFilter = gatewayv1.GRPCRouteFilter{
					Type: gatewayv1.GRPCRouteFilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Set:    httpFilter.ResponseHeaderModifier.Set,
						Add:    httpFilter.ResponseHeaderModifier.Add,
						Remove: httpFilter.ResponseHeaderModifier.Remove,
					},
				}
				grpcFilters = append(grpcFilters, grpcFilter)
			}

		// These HTTP filter types are not applicable to gRPC and are skipped
		case gatewayv1.HTTPRouteFilterRequestRedirect:
			notify(notifications.WarningNotification, "RequestRedirect is not applicable to gRPC")
		case gatewayv1.HTTPRouteFilterURLRewrite:
			notify(notifications.WarningNotification, "URLRewrite is not applicable to gRPC")
		case gatewayv1.HTTPRouteFilterRequestMirror:
			notify(notifications.WarningNotification, "RequestMirror is not applicable to gRPC")
		case gatewayv1.HTTPRouteFilterExtensionRef:
			notify(notifications.WarningNotification, "ExtensionRef filters are not converted to gRPC equivalents")
		default:
			notify(notifications.WarningNotification, "Unknown HTTPRouteFilter type: "+string(httpFilter.Type))
		}
	}

	return grpcFilters
}

// removeGRPCRulesFromHTTPRoute removes HTTPRoute rules that target gRPC services
func removeGRPCRulesFromHTTPRoute(httpRoute *gatewayv1.HTTPRoute, grpcServiceSet map[string]struct{}) []gatewayv1.HTTPRouteRule {
	var remainingRules []gatewayv1.HTTPRouteRule

	for _, rule := range httpRoute.Spec.Rules {
		var remainingBackendRefs []gatewayv1.HTTPBackendRef

		// Check each backend ref in the rule
		for _, backendRef := range rule.BackendRefs {
			serviceName := string(backendRef.BackendRef.BackendObjectReference.Name)
			// Only keep backend refs that are NOT gRPC services
			if _, isGRPCService := grpcServiceSet[serviceName]; !isGRPCService {
				remainingBackendRefs = append(remainingBackendRefs, backendRef)
			}
		}

		// If any backend refs remain, keep the rule
		if len(remainingBackendRefs) > 0 {
			rule.BackendRefs = remainingBackendRefs
			remainingRules = append(remainingRules, rule)
		}
	}

	return remainingRules
}
