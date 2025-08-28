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

package common

import (
	"fmt"
	"regexp"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
)

func GetIngressClass(ingress networkingv1.Ingress) string {
	var ingressClass string

	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName != "" {
		ingressClass = *ingress.Spec.IngressClassName
	} else if _, ok := ingress.Annotations[networkingv1beta1.AnnotationIngressClass]; ok {
		ingressClass = ingress.Annotations[networkingv1beta1.AnnotationIngressClass]
	} else {
		ingressClass = ""
	}

	return ingressClass
}

type IngressRuleGroup struct {
	Namespace    string
	Name         string
	IngressClass string
	Host         string
	TLS          []networkingv1.IngressTLS
	Rules        []Rule
}

type Rule struct {
	Ingress     networkingv1.Ingress
	IngressRule networkingv1.IngressRule
}

func GetRuleGroups(ingresses []networkingv1.Ingress) map[string]IngressRuleGroup {
	ruleGroups := make(map[string]IngressRuleGroup)

	for _, ingress := range ingresses {
		ingressClass := GetIngressClass(ingress)

		for _, rule := range ingress.Spec.Rules {

			rgKey := fmt.Sprintf("%s/%s/%s", ingress.Namespace, ingressClass, rule.Host)
			rg, ok := ruleGroups[rgKey]
			if !ok {
				rg = IngressRuleGroup{
					Namespace:    ingress.Namespace,
					Name:         ingress.Name,
					IngressClass: ingressClass,
					Host:         rule.Host,
				}
				ruleGroups[rgKey] = rg
			}
			rg.TLS = append(rg.TLS, ingress.Spec.TLS...)
			rg.Rules = append(rg.Rules, Rule{
				Ingress:     ingress,
				IngressRule: rule,
			})

			ruleGroups[rgKey] = rg
		}

	}

	return ruleGroups
}

func NameFromHost(host string) string {
	// replace all special chars with -
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	step1 := reg.ReplaceAllString(host, "-")
	// remove all - at start of string
	reg2, _ := regexp.Compile("^[^a-zA-Z0-9]+")
	step2 := reg2.ReplaceAllString(step1, "")
	// if nothing left, return "all-hosts"
	if len(host) == 0 || host == "*" {
		return "all-hosts"
	}
	return step2
}

func RouteName(ingressName, host string) string {
	return fmt.Sprintf("%s-%s", ingressName, NameFromHost(host))
}

func ToBackendRef(namespace string, ib networkingv1.IngressBackend, servicePorts map[types.NamespacedName]map[string]int32, path *field.Path) (*gatewayv1.BackendRef, *field.Error) {
	if ib.Service != nil {
		if ib.Service.Port.Name == "" {
			return &gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Name: gatewayv1.ObjectName(ib.Service.Name),
					Port: (*gatewayv1.PortNumber)(&ib.Service.Port.Number),
				},
			}, nil
		}

		portNumber, ok := servicePorts[types.NamespacedName{Namespace: namespace, Name: ib.Service.Name}][ib.Service.Port.Name]
		if !ok {
			fieldPath := path.Child("service", "port")
			return nil, field.Invalid(fieldPath, "name", fmt.Sprintf("cannot find port with name %s in service %s", ib.Service.Port.Name, ib.Service.Name))
		}

		return &gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Name: gatewayv1.ObjectName(ib.Service.Name),
				Port: (*gatewayv1.PortNumber)(&portNumber),
			},
		}, nil
	}
	return &gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Group: (*gatewayv1.Group)(ib.Resource.APIGroup),
			Kind:  (*gatewayv1.Kind)(&ib.Resource.Kind),
			Name:  gatewayv1.ObjectName(ib.Resource.Name),
		},
	}, nil
}

type orderedIngressPathsByMatchKey struct {
	keys []pathMatchKey
	data map[pathMatchKey][]ingressPath
}

func groupIngressPathsByMatchKey(rules []ingressRule) orderedIngressPathsByMatchKey {
	// we track the keys in an additional slice in order to preserve the rules order
	ingressPathsByMatchKey := orderedIngressPathsByMatchKey{
		keys: []pathMatchKey{},
		data: map[pathMatchKey][]ingressPath{},
	}

	for i, ir := range rules {
		for j, path := range ir.rule.HTTP.Paths {
			ip := ingressPath{ruleIdx: i, pathIdx: j, ruleType: "http", path: path}
			pmKey := getPathMatchKey(ip)
			if _, ok := ingressPathsByMatchKey.data[pmKey]; !ok {
				ingressPathsByMatchKey.keys = append(ingressPathsByMatchKey.keys, pmKey)
			}
			ingressPathsByMatchKey.data[pmKey] = append(ingressPathsByMatchKey.data[pmKey], ip)
		}
	}
	return ingressPathsByMatchKey
}

func GroupServicePortsByPortName(services map[types.NamespacedName]*apiv1.Service) map[types.NamespacedName]map[string]int32 {
	servicePorts := map[types.NamespacedName]map[string]int32{}

	for namespacedName, service := range services {
		servicePorts[namespacedName] = map[string]int32{}
		for _, port := range service.Spec.Ports {
			servicePorts[namespacedName][port.Name] = port.Port
		}
	}

	return servicePorts
}

func PtrTo[T any](a T) *T {
	return &a
}

type uniqueBackendRefsKey struct {
	Name      gatewayv1.ObjectName
	Namespace gatewayv1.Namespace
	Port      gatewayv1.PortNumber
	Group     gatewayv1.Group
	Kind      gatewayv1.Kind
}

// removeBackendRefsDuplicates removes duplicate backendRefs from a list of backendRefs.
func removeBackendRefsDuplicates(backendRefs []gatewayv1.HTTPBackendRef) []gatewayv1.HTTPBackendRef {
	var uniqueBackendRefs []gatewayv1.HTTPBackendRef
	uniqueKeys := map[uniqueBackendRefsKey]struct{}{}

	for _, backendRef := range backendRefs {
		var k uniqueBackendRefsKey

		group := gatewayv1.Group("")
		kind := gatewayv1.Kind("Service")

		if backendRef.Group != nil && *backendRef.Group != "core" {
			group = *backendRef.Group
		}

		if backendRef.Kind != nil {
			kind = *backendRef.Kind
		}

		k.Name = backendRef.Name
		k.Group = group
		k.Kind = kind

		if backendRef.Port != nil {
			k.Port = *backendRef.Port
		}

		if _, exists := uniqueKeys[k]; exists {
			continue
		}

		uniqueKeys[k] = struct{}{}
		uniqueBackendRefs = append(uniqueBackendRefs, backendRef)
	}
	return uniqueBackendRefs
}

// ParseGRPCServiceMethod parses gRPC service and method from HTTP path
func ParseGRPCServiceMethod(path string) (service, method string) {
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

// GRPCFilterConversionResult holds the result of converting HTTP filters to gRPC filters
type GRPCFilterConversionResult struct {
	GRPCFilters      []gatewayv1.GRPCRouteFilter
	UnsupportedTypes []gatewayv1.HTTPRouteFilterType
}

// ConvertHTTPFiltersToGRPCFilters converts a list of HTTPRoute filters to GRPCRoute filters
// Returns both the converted filters and a list of unsupported filter types for notification
func ConvertHTTPFiltersToGRPCFilters(httpFilters []gatewayv1.HTTPRouteFilter) GRPCFilterConversionResult {
	if len(httpFilters) == 0 {
		return GRPCFilterConversionResult{
			GRPCFilters:      []gatewayv1.GRPCRouteFilter{},
			UnsupportedTypes: []gatewayv1.HTTPRouteFilterType{},
		}
	}

	var grpcFilters []gatewayv1.GRPCRouteFilter
	var unsupportedTypes []gatewayv1.HTTPRouteFilterType

	for _, httpFilter := range httpFilters {
		switch httpFilter.Type {
		case gatewayv1.HTTPRouteFilterRequestHeaderModifier:
			if httpFilter.RequestHeaderModifier != nil {
				grpcFilter := gatewayv1.GRPCRouteFilter{
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
				grpcFilter := gatewayv1.GRPCRouteFilter{
					Type: gatewayv1.GRPCRouteFilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
						Set:    httpFilter.ResponseHeaderModifier.Set,
						Add:    httpFilter.ResponseHeaderModifier.Add,
						Remove: httpFilter.ResponseHeaderModifier.Remove,
					},
				}
				grpcFilters = append(grpcFilters, grpcFilter)
			}

		// These HTTP filter types are not applicable to gRPC
		case gatewayv1.HTTPRouteFilterRequestRedirect:
			unsupportedTypes = append(unsupportedTypes, httpFilter.Type)
		case gatewayv1.HTTPRouteFilterURLRewrite:
			unsupportedTypes = append(unsupportedTypes, httpFilter.Type)
		case gatewayv1.HTTPRouteFilterRequestMirror:
			unsupportedTypes = append(unsupportedTypes, httpFilter.Type)
		case gatewayv1.HTTPRouteFilterExtensionRef:
			unsupportedTypes = append(unsupportedTypes, httpFilter.Type)
		default:
			unsupportedTypes = append(unsupportedTypes, httpFilter.Type)
		}
	}

	// Ensure we return empty slices instead of nil
	if grpcFilters == nil {
		grpcFilters = []gatewayv1.GRPCRouteFilter{}
	}
	if unsupportedTypes == nil {
		unsupportedTypes = []gatewayv1.HTTPRouteFilterType{}
	}

	return GRPCFilterConversionResult{
		GRPCFilters:      grpcFilters,
		UnsupportedTypes: unsupportedTypes,
	}
}

// RemoveGRPCRulesFromHTTPRoute removes HTTPRoute rules that target gRPC services.
// When route rules are converted from HTTP to gRPC routes, this function cleans up the original
// HTTPRoute by removing backend references to those gRPC services, preventing duplicate routing.
func RemoveGRPCRulesFromHTTPRoute(httpRoute *gatewayv1.HTTPRoute, grpcServiceSet map[string]struct{}) []gatewayv1.HTTPRouteRule {
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

// CreateBackendTLSPolicy creates a BackendTLSPolicy for the given service
func CreateBackendTLSPolicy(namespace, policyName, serviceName string) gatewayv1alpha3.BackendTLSPolicy {

	// TODO: Migrate BackendTLSPolicy from gatewayv1alpha3 to gatewayv1 for Gateway API 1.4
	// See: https://github.com/kubernetes-sigs/ingress2gateway/issues/236
	return gatewayv1alpha3.BackendTLSPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha3.GroupVersion.String(),
			Kind:       "BackendTLSPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
		Spec: gatewayv1alpha3.BackendTLSPolicySpec{
			TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReferenceWithSectionName{
				{
					LocalPolicyTargetReference: gatewayv1alpha2.LocalPolicyTargetReference{
						Group: "", // Core group
						Kind:  "Service",
						Name:  gatewayv1.ObjectName(serviceName),
					},
				},
			},
			Validation: gatewayv1alpha3.BackendTLSPolicyValidation{
				// Note: WellKnownCACertificates and Hostname fields are intentionally left empty
				// These fields must be manually configured based on your backend service's TLS setup
			},
		},
	}
}

// BackendTLSPolicyName returns the generated name for a BackendTLSPolicy
// Providers can use this function or create their own naming scheme
func BackendTLSPolicyName(ingressName, serviceName string) string {
	return fmt.Sprintf("%s-%s-backend-tls", ingressName, serviceName)
}
