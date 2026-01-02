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
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type Protocol string

const (
	HTTP Protocol = "http"
	GRPC Protocol = "grpc"
)

// ToIR converts the received ingresses to providerir.ProviderIR without taking into
// consideration any provider specific logic.
func ToIR(httpIngresses []networkingv1.Ingress, grpcIngresses []networkingv1.Ingress, servicePorts map[types.NamespacedName]map[string]int32, options i2gw.ProviderImplementationSpecificOptions) (providerir.ProviderIR, field.ErrorList) {
	aggregator := ingressAggregator{
		ruleGroups:   map[ruleGroupKey]*ingressRuleGroup{},
		servicePorts: servicePorts,
	}

	var errs field.ErrorList
	for _, ingress := range httpIngresses {
		aggregator.addIngress(ingress, string(HTTP))
	}

	for _, ingress := range grpcIngresses {
		aggregator.addIngress(ingress, string(GRPC))
	}

	httproutes, grpcroutes, gateways, errs := aggregator.toRoutesAndGateways(options)
	if len(errs) > 0 {
		return providerir.ProviderIR{}, errs
	}

	httpRouteByKey := make(map[types.NamespacedName]providerir.HTTPRouteContext)
	for _, routeWithSources := range httproutes {
		key := types.NamespacedName{Namespace: routeWithSources.route.Namespace, Name: routeWithSources.route.Name}
		httpRouteByKey[key] = providerir.HTTPRouteContext{
			HTTPRoute:          routeWithSources.route,
			RuleBackendSources: routeWithSources.sources,
		}
	}

	grpcRouteByKey := make(map[types.NamespacedName]providerir.GRPCRouteContext)
	for _, routeWithSources := range grpcroutes {
		key := types.NamespacedName{Namespace: routeWithSources.route.Namespace, Name: routeWithSources.route.Name}
		grpcRouteByKey[key] = providerir.GRPCRouteContext{
			GRPCRoute:          routeWithSources.route,
			RuleBackendSources: routeWithSources.sources,
		}
	}

	gatewayByKey := make(map[types.NamespacedName]providerir.GatewayContext)
	for _, gateway := range gateways {
		key := types.NamespacedName{Namespace: gateway.Namespace, Name: gateway.Name}
		gatewayByKey[key] = providerir.GatewayContext{Gateway: gateway}
	}

	return providerir.ProviderIR{
		Gateways:           gatewayByKey,
		HTTPRoutes:         httpRouteByKey,
		Services:           make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR),
		GatewayClasses:     make(map[types.NamespacedName]gatewayv1.GatewayClass),
		TLSRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		UDPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.UDPRoute),
		GRPCRoutes:         grpcRouteByKey,
		BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1.BackendTLSPolicy),
		ReferenceGrants:    make(map[types.NamespacedName]gatewayv1beta1.ReferenceGrant),
	}, nil
}

var (
	GatewayGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "Gateway",
	}

	HTTPRouteGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "HTTPRoute",
	}

	TLSRouteGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1alpha2",
		Kind:    "TLSRoute",
	}

	TCPRouteGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1alpha2",
		Kind:    "TCPRoute",
	}

	ReferenceGrantGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1beta1",
		Kind:    "ReferenceGrant",
	}

	GRPCRouteGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "GRPCRoute",
	}
)

type ruleGroupKey string

type ingressAggregator struct {
	ruleGroups      map[ruleGroupKey]*ingressRuleGroup
	listeners       map[string][]listenerWithProtocol
	defaultBackends []ingressDefaultBackend
	servicePorts    map[types.NamespacedName]map[string]int32
}

type listenerWithProtocol struct {
	listener gatewayv1.Listener
	protocol string
}

type pathMatchKey string

type ingressRuleGroup struct {
	namespace    string
	name         string
	ingressClass string
	host         string
	tls          []networkingv1.IngressTLS
	rules        []ingressRule
	protocol     string
}

type ingressRule struct {
	// Source tracking
	ingress *networkingv1.Ingress

	rule networkingv1.IngressRule
}

type ingressDefaultBackend struct {
	name          string
	namespace     string
	ingressClass  string
	backend       networkingv1.IngressBackend
	sourceIngress *networkingv1.Ingress
}

type ingressPath struct {
	// These are for source error propagation
	ruleIdx       int
	pathIdx       int
	ruleType      string
	path          networkingv1.HTTPIngressPath
	sourceIngress *networkingv1.Ingress
}

func (a *ingressAggregator) addIngress(ingress networkingv1.Ingress, protocol string) {
	ingressClass := GetIngressClass(ingress)
	for _, rule := range ingress.Spec.Rules {
		a.addIngressRule(ingress, ingressClass, rule, protocol)
	}
	if ingress.Spec.DefaultBackend != nil {
		a.defaultBackends = append(a.defaultBackends, ingressDefaultBackend{
			name:          ingress.Name,
			namespace:     ingress.Namespace,
			ingressClass:  ingressClass,
			backend:       *ingress.Spec.DefaultBackend,
			sourceIngress: &ingress,
		})
	}
}

func (a *ingressAggregator) addIngressRule(ingress networkingv1.Ingress, ingressClass string, rule networkingv1.IngressRule, protocol string) {
	rgKey := ruleGroupKey(fmt.Sprintf("%s/%s/%s/%s", ingress.Namespace, ingressClass, rule.Host, protocol))
	rg, ok := a.ruleGroups[rgKey]
	if !ok {
		rg = &ingressRuleGroup{
			namespace:    ingress.Namespace,
			name:         ingress.Name,
			ingressClass: ingressClass,
			host:         rule.Host,
			protocol:     protocol,
		}
		a.ruleGroups[rgKey] = rg
	}
	if len(ingress.Spec.TLS) > 0 {
		rg.tls = append(rg.tls, ingress.Spec.TLS...)
	}
	rg.rules = append(rg.rules, ingressRule{
		ingress: &ingress,
		rule:    rule,
	})
}

type httpRouteWithSources struct {
	route   gatewayv1.HTTPRoute
	sources [][]providerir.BackendSource
}

type grpcRouteWithSources struct {
	route   gatewayv1.GRPCRoute
	sources [][]providerir.BackendSource
}

func (a *ingressAggregator) toRoutesAndGateways(options i2gw.ProviderImplementationSpecificOptions) ([]httpRouteWithSources, []grpcRouteWithSources, []gatewayv1.Gateway, field.ErrorList) {
	var httpRoutes []httpRouteWithSources
	var grpcRoutes []grpcRouteWithSources
	var errors field.ErrorList
	listenersByNamespacedGateway := map[string][]listenerWithProtocol{}

	// Sort the rulegroups to iterate the map in a sorted order.
	ruleGroupsKeys := make([]ruleGroupKey, 0, len(a.ruleGroups))
	for k := range a.ruleGroups {
		ruleGroupsKeys = append(ruleGroupsKeys, k)
	}

	slices.SortFunc(ruleGroupsKeys, func(a, b ruleGroupKey) int {
		return cmp.Compare(a, b)
	})

	for _, rgk := range ruleGroupsKeys {
		rg := a.ruleGroups[rgk]
		listener := gatewayv1.Listener{}
		if rg.host != "" {
			listener.Hostname = (*gatewayv1.Hostname)(&rg.host)
		} else if len(rg.tls) == 1 && len(rg.tls[0].Hosts) == 1 {
			listener.Hostname = (*gatewayv1.Hostname)(&rg.tls[0].Hosts[0])
		}
		if len(rg.tls) > 0 {
			listener.TLS = &gatewayv1.ListenerTLSConfig{}
		}
		for _, tls := range rg.tls {
			listener.TLS.CertificateRefs = append(listener.TLS.CertificateRefs,
				gatewayv1.SecretObjectReference{Name: gatewayv1.ObjectName(tls.SecretName)})
		}
		gwKey := fmt.Sprintf("%s/%s", rg.namespace, rg.ingressClass)
		listenersByNamespacedGateway[gwKey] = append(listenersByNamespacedGateway[gwKey], listenerWithProtocol{listener: listener, protocol: rg.protocol})
		if rg.protocol == string(HTTP) {
			httpRoute, sources, errs := rg.toHTTPRoute(a.servicePorts, options)
			httpRoutes = append(httpRoutes, httpRouteWithSources{route: httpRoute, sources: sources})
			errors = append(errors, errs...)
		} else {
			grpcRoute, sources, errs := rg.toGRPCRoute(a.servicePorts, options)
			grpcRoutes = append(grpcRoutes, grpcRouteWithSources{route: grpcRoute, sources: sources})
			errors = append(errors, errs...)
		}
	}

	for i, db := range a.defaultBackends {
		httpRoute := gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-default-backend", db.name),
				Namespace: db.namespace,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{{
						Name: gatewayv1.ObjectName(db.ingressClass),
					}},
				},
			},
			Status: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{},
				},
			},
		}
		httpRoute.SetGroupVersionKind(HTTPRouteGVK)

		// We create an HTTPRoute with a single rule and a single backend.
		backendRef, err := ToBackendRef(db.namespace, db.backend, a.servicePorts, field.NewPath(db.name, "paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
		} else {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1.HTTPRouteRule{
				BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: *backendRef}},
			})
		}
		// Set the single source for this default backend.
		sources := [][]providerir.BackendSource{
			{
				{
					Ingress:        db.sourceIngress,
					DefaultBackend: &db.backend,
				},
			},
		}
		httpRoutes = append(httpRoutes, httpRouteWithSources{route: httpRoute, sources: sources})
	}

	gatewaysByKey := map[string]*gatewayv1.Gateway{}
	for gwKey, listeners := range listenersByNamespacedGateway {
		parts := strings.Split(gwKey, "/")
		if len(parts) != 2 {
			errors = append(errors, field.Invalid(field.NewPath(""), "", fmt.Sprintf("error generating Gateway listeners for key: %s", gwKey)))
			continue
		}
		gateway := gatewaysByKey[gwKey]
		if gateway == nil {
			gateway = &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: parts[0],
					Name:      parts[1],
				},
				Spec: gatewayv1.GatewaySpec{
					GatewayClassName: gatewayv1.ObjectName(parts[1]),
				},
			}
			gateway.SetGroupVersionKind(GatewayGVK)
			gatewaysByKey[gwKey] = gateway
		}
		for _, lwp := range listeners {
			listener := lwp.listener
			var listenerNamePrefix string
			if listener.Hostname != nil && *listener.Hostname != "" {
				listenerNamePrefix = fmt.Sprintf("%s-", NameFromHost(string(*listener.Hostname)))
			}

			gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1.Listener{
				Name:     gatewayv1.SectionName(fmt.Sprintf("%s%s", listenerNamePrefix, strings.ToLower(lwp.protocol))),
				Hostname: listener.Hostname,
				Port:     80,
				Protocol: gatewayv1.HTTPProtocolType,
			})
			if listener.TLS != nil {
				gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1.Listener{
					Name:     gatewayv1.SectionName(fmt.Sprintf("%shttps", listenerNamePrefix)),
					Hostname: listener.Hostname,
					Port:     443,
					Protocol: gatewayv1.HTTPSProtocolType,
					TLS:      listener.TLS,
				})
			}
		}
	}

	var gateways []gatewayv1.Gateway
	for _, gw := range gatewaysByKey {
		gateways = append(gateways, *gw)
	}

	return httpRoutes, grpcRoutes, gateways, errors
}

func (rg *ingressRuleGroup) toHTTPRoute(servicePorts map[types.NamespacedName]map[string]int32, options i2gw.ProviderImplementationSpecificOptions) (gatewayv1.HTTPRoute, [][]providerir.BackendSource, field.ErrorList) {
	ingressPathsByMatchKey := groupIngressPathsByMatchKey(rg.rules)
	httpRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RouteName(rg.name, rg.host),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{},
		Status: gatewayv1.HTTPRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{},
			},
		},
	}
	httpRoute.SetGroupVersionKind(HTTPRouteGVK)

	if rg.ingressClass != "" {
		httpRoute.Spec.ParentRefs = []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName(rg.ingressClass)}}
	}
	if rg.host != "" {
		httpRoute.Spec.Hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(rg.host)}
	}

	var errors field.ErrorList
	var allRuleBackendSources [][]providerir.BackendSource

	for _, key := range ingressPathsByMatchKey.keys {
		paths := ingressPathsByMatchKey.data[key]
		path := paths[0]
		fieldPath := field.NewPath("spec", "rules").Index(path.ruleIdx).Child(path.ruleType).Child("paths").Index(path.pathIdx)
		match, err := toHTTPRouteMatch(path.path, fieldPath, options.ToImplementationSpecificHTTPPathTypeMatch)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		hrRule := gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{*match},
		}

		backendRefs, sources, errs := rg.configureBackendRef(servicePorts, paths)
		errors = append(errors, errs...)
		hrRule.BackendRefs = backendRefs

		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, hrRule)
		allRuleBackendSources = append(allRuleBackendSources, sources)
	}

	return httpRoute, allRuleBackendSources, errors
}

func (rg *ingressRuleGroup) configureBackendRef(servicePorts map[types.NamespacedName]map[string]int32, paths []ingressPath) ([]gatewayv1.HTTPBackendRef, []providerir.BackendSource, field.ErrorList) {
	var errors field.ErrorList
	var backendRefs []gatewayv1.HTTPBackendRef
	var sources []providerir.BackendSource

	for i, path := range paths {
		backendRef, err := ToBackendRef(rg.namespace, path.path.Backend, servicePorts, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		backendRefs = append(backendRefs, gatewayv1.HTTPBackendRef{BackendRef: *backendRef})

		// Track source for this backend
		sources = append(sources, providerir.BackendSource{
			Ingress: path.sourceIngress,
			Path:    &path.path,
		})
	}

	// keep duplicates as they might have different sources.
	return backendRefs, sources, errors
}

func getPathMatchKey(ip ingressPath) pathMatchKey {
	var pathType string
	if ip.path.PathType != nil {
		pathType = string(*ip.path.PathType)
	}
	return pathMatchKey(fmt.Sprintf("%s/%s", pathType, ip.path.Path))
}

func toHTTPRouteMatch(routePath networkingv1.HTTPIngressPath, path *field.Path, toImplementationSpecificPathMatch i2gw.ImplementationSpecificHTTPPathTypeMatchConverter) (*gatewayv1.HTTPRouteMatch, *field.Error) {
	pmPrefix := gatewayv1.PathMatchPathPrefix
	pmExact := gatewayv1.PathMatchExact

	match := &gatewayv1.HTTPRouteMatch{Path: &gatewayv1.HTTPPathMatch{Value: &routePath.Path}}

	if routePath.PathType == nil {
		return nil, field.Invalid(path.Child("pathType"), routePath.PathType, "pathType is required")
	}

	switch *routePath.PathType {
	case networkingv1.PathTypePrefix:
		match.Path.Type = &pmPrefix
	case networkingv1.PathTypeExact:
		match.Path.Type = &pmExact
	// In case the path type is ImplementationSpecific, the path value and type
	// will be set by the provider-specific customization function. If such function
	// is not given by the provider, an error is returned.
	case networkingv1.PathTypeImplementationSpecific:
		if toImplementationSpecificPathMatch != nil {
			toImplementationSpecificPathMatch(match.Path)
		} else {
			return nil, field.Invalid(path.Child("pathType"), routePath.PathType, "implementationSpecific path type is not supported in generic translation, and your provider does not provide custom support to translate it")
		}
	default:
		// default should never hit, as all the possible cases are already checked
		// via proper switch cases.
		return nil, field.Invalid(path.Child("pathType"), match.Path.Type, fmt.Sprintf("unsupported path match type: %s", *match.Path.Type))

	}

	return match, nil
}

func (rg *ingressRuleGroup) toGRPCRoute(servicePorts map[types.NamespacedName]map[string]int32, options i2gw.ProviderImplementationSpecificOptions) (gatewayv1.GRPCRoute, [][]providerir.BackendSource, field.ErrorList) {
	// GRPCRoutes don't have path matches in the same way Ingress does (Method/Service vs URI Path).
	// For standard Ingress -> GRPCRoute conversion, we usually Map all backends to a catch-all Rule
	// or we could try to map paths if they look like gRPC methods, but that's flaky.
	// We will follow the logic of "Host -> GRPCRoute with BackendRefs from all paths".

	grpcRoute := gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RouteName(rg.name, rg.host),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1.GRPCRouteSpec{},
	}
	grpcRoute.SetGroupVersionKind(GRPCRouteGVK)

	if rg.ingressClass != "" {
		grpcRoute.Spec.ParentRefs = []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName(rg.ingressClass)}}
	}
	if rg.host != "" {
		grpcRoute.Spec.Hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(rg.host)}
	}

	var errors field.ErrorList

	// Collect all backendRefs from all rules/paths in this group
	var allBackendRefs []gatewayv1.GRPCBackendRef
	var allSources []providerir.BackendSource

	for _, rule := range rg.rules {
		for i, path := range rule.rule.HTTP.Paths {
			// Convert to BackendRef
			backendRef, err := ToBackendRef(rg.namespace, path.Backend, servicePorts, field.NewPath("spec", "rules", "http", "paths").Index(i))
			if err != nil {
				errors = append(errors, err)
				continue
			}

			grpcBackendRef := gatewayv1.GRPCBackendRef{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Group:     backendRef.Group,
						Kind:      backendRef.Kind,
						Name:      backendRef.Name,
						Port:      backendRef.Port,
						Namespace: backendRef.Namespace,
					},
					Weight: backendRef.Weight,
				},
			}
			allBackendRefs = append(allBackendRefs, grpcBackendRef)

			// Track source for this backend
			allSources = append(allSources, providerir.BackendSource{
				Ingress: rule.ingress,
				Path:    &path,
			})
		}
	}

	var allRuleBackendSources [][]providerir.BackendSource
	if len(allBackendRefs) > 0 {
		grpcRoute.Spec.Rules = []gatewayv1.GRPCRouteRule{
			{
				BackendRefs: allBackendRefs,
				// No matches -> Catch-all for this host
			},
		}
		// All backends go into a single rule, so wrap all sources in one slice
		allRuleBackendSources = [][]providerir.BackendSource{allSources}
	}

	return grpcRoute, allRuleBackendSources, errors
}
