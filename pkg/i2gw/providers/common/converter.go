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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ToIR converts the received ingresses to intermediate.IR without taking into
// consideration any provider specific logic.
func ToIR(ingresses []networkingv1.Ingress, servicePorts map[types.NamespacedName]map[string]int32, options i2gw.ProviderImplementationSpecificOptions) (intermediate.IR, field.ErrorList) {
	aggregator := ingressAggregator{
		ruleGroups:   map[ruleGroupKey]*ingressRuleGroup{},
		servicePorts: servicePorts,
	}

	var errs field.ErrorList
	for _, ingress := range ingresses {
		aggregator.addIngress(ingress)
	}
	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}

	routes, gateways, errs := aggregator.toHTTPRoutesAndGateways(options)
	if len(errs) > 0 {
		return intermediate.IR{}, errs
	}

	routeByKey := make(map[types.NamespacedName]intermediate.HTTPRouteContext)
	for _, route := range routes {
		key := types.NamespacedName{Namespace: route.Namespace, Name: route.Name}
		routeByKey[key] = intermediate.HTTPRouteContext{HTTPRoute: route}
	}

	gatewayByKey := make(map[types.NamespacedName]intermediate.GatewayContext)
	for _, gateway := range gateways {
		key := types.NamespacedName{Namespace: gateway.Namespace, Name: gateway.Name}
		gatewayByKey[key] = intermediate.GatewayContext{Gateway: gateway}
	}

	return intermediate.IR{
		Gateways:           gatewayByKey,
		HTTPRoutes:         routeByKey,
		Services:           make(map[types.NamespacedName]intermediate.ProviderSpecificServiceIR),
		GatewayClasses:     make(map[types.NamespacedName]gatewayv1.GatewayClass),
		TLSRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		UDPRoutes:          make(map[types.NamespacedName]gatewayv1alpha2.UDPRoute),
		GRPCRoutes:         make(map[types.NamespacedName]gatewayv1.GRPCRoute),
		BackendTLSPolicies: make(map[types.NamespacedName]gatewayv1alpha3.BackendTLSPolicy),
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
)

type ruleGroupKey string

type ingressAggregator struct {
	ruleGroups      map[ruleGroupKey]*ingressRuleGroup
	defaultBackends []ingressDefaultBackend
	servicePorts    map[types.NamespacedName]map[string]int32
}

type pathMatchKey string

type ingressRuleGroup struct {
	Namespace    string
	Name         string
	IngressClass string
	Host         string
	TLS          []networkingv1.IngressTLS
	Rules        []Rule
}

type Rule struct {
	Ingress     *networkingv1.Ingress
	IngressRule *networkingv1.IngressRule
}

type ingressRule struct {
	rule networkingv1.IngressRule
}

type ingressDefaultBackend struct {
	name         string
	namespace    string
	ingressClass string
	ingress      *networkingv1.Ingress
	backend      *networkingv1.IngressBackend
}

func (a *ingressAggregator) addIngress(ingress networkingv1.Ingress) {
	ingressClass := GetIngressClass(ingress)
	for _, rule := range ingress.Spec.Rules {
		a.addIngressRule(ingress.Namespace, ingress.Name, ingressClass, rule, &ingress)
	}
	if ingress.Spec.DefaultBackend != nil {
		a.defaultBackends = append(a.defaultBackends, ingressDefaultBackend{
			name:         ingress.Name,
			namespace:    ingress.Namespace,
			ingressClass: ingressClass,
			ingress:      &ingress,
			backend:      ingress.Spec.DefaultBackend,
		})
	}
}

func (a *ingressAggregator) addIngressRule(namespace, name, ingressClass string, rule networkingv1.IngressRule, ingress *networkingv1.Ingress) {
	rgKey := ruleGroupKey(fmt.Sprintf("%s/%s/%s", namespace, ingressClass, rule.Host))
	rg, ok := a.ruleGroups[rgKey]
	if !ok {
		rg = &ingressRuleGroup{
			Namespace:    namespace,
			Name:         name,
			IngressClass: ingressClass,
			Host:         rule.Host,
		}
		a.ruleGroups[rgKey] = rg
	}
	if len(ingress.Spec.TLS) > 0 {
		rg.TLS = append(rg.TLS, ingress.Spec.TLS...)
	}
	rg.Rules = append(rg.Rules, Rule{IngressRule: &rule, Ingress: ingress})
}

func (a *ingressAggregator) toHTTPRoutesAndGateways(options i2gw.ProviderImplementationSpecificOptions) ([]gatewayv1.HTTPRoute, []gatewayv1.Gateway, field.ErrorList) {
	var httpRoutes []gatewayv1.HTTPRoute
	var errors field.ErrorList
	listenersByNamespacedGateway := map[string][]gatewayv1.Listener{}

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
		if rg.Host != "" {
			listener.Hostname = (*gatewayv1.Hostname)(&rg.Host)
		} else if len(rg.TLS) == 1 && len(rg.TLS[0].Hosts) == 1 {
			listener.Hostname = (*gatewayv1.Hostname)(&rg.TLS[0].Hosts[0])
		}
		if len(rg.TLS) > 0 {
			listener.TLS = &gatewayv1.GatewayTLSConfig{}
		}
		for _, tls := range rg.TLS {
			listener.TLS.CertificateRefs = append(listener.TLS.CertificateRefs,
				gatewayv1.SecretObjectReference{Name: gatewayv1.ObjectName(tls.SecretName)})
		}
		gwKey := fmt.Sprintf("%s/%s", rg.Namespace, rg.IngressClass)
		listenersByNamespacedGateway[gwKey] = append(listenersByNamespacedGateway[gwKey], listener)
		httpRoute, errs := rg.toHTTPRoute(a.servicePorts, options)
		httpRoutes = append(httpRoutes, httpRoute)
		errors = append(errors, errs...)
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

		backendRef, err := ToBackendRef(db.namespace, *db.backend, a.servicePorts, field.NewPath(db.name, "paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
		} else {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1.HTTPRouteRule{
				BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: *backendRef}},
			})
		}

		httpRoutes = append(httpRoutes, httpRoute)
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
		for _, listener := range listeners {
			var listenerNamePrefix string
			if listener.Hostname != nil && *listener.Hostname != "" {
				listenerNamePrefix = fmt.Sprintf("%s-", NameFromHost(string(*listener.Hostname)))
			}

			gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1.Listener{
				Name:     gatewayv1.SectionName(fmt.Sprintf("%shttp", listenerNamePrefix)),
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

	return httpRoutes, gateways, errors
}

func (rg *ingressRuleGroup) toHTTPRoute(servicePorts map[types.NamespacedName]map[string]int32, options i2gw.ProviderImplementationSpecificOptions) (gatewayv1.HTTPRoute, field.ErrorList) {
	ingressPathsByMatchKey := groupIngressPathsByMatchKey(rg.Rules)
	httpRoute := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RouteName(rg.Name, rg.Host),
			Namespace: rg.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{},
		Status: gatewayv1.HTTPRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{},
			},
		},
	}
	httpRoute.SetGroupVersionKind(HTTPRouteGVK)

	if rg.IngressClass != "" {
		httpRoute.Spec.ParentRefs = []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName(rg.IngressClass)}}
	}
	if rg.Host != "" {
		httpRoute.Spec.Hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(rg.Host)}
	}

	var errors field.ErrorList
	for _, key := range ingressPathsByMatchKey.keys {
		paths := ingressPathsByMatchKey.data[key]
		path := paths[0]
		fieldPath := field.NewPath("spec", "rules").Index(path.RuleIdx).Child(path.RuleType).Child("paths").Index(path.PathIdx)

		if options.ToImplementationSpecificRules != nil {
			hrRules, errs := options.ToImplementationSpecificRules(paths, fieldPath, servicePorts, rg.Namespace)
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, hrRules...)
			errors = append(errors, errs...)
			continue
		}
		// default translation logic
		match, err := ToHTTPRouteMatch(path.Path, fieldPath, options.ToImplementationSpecificHTTPPathTypeMatch)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		hrRule := gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{*match},
		}

		backendRefs, errs := ConfigureBackendRef(servicePorts, paths, rg.Namespace)
		errors = append(errors, errs...)
		hrRule.BackendRefs = backendRefs

		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, hrRule)
	}

	return httpRoute, errors
}

func getPathMatchKey(ip i2gw.IngressPath) pathMatchKey {
	var pathType string
	if ip.Path.PathType != nil {
		pathType = string(*ip.Path.PathType)
	}
	return pathMatchKey(fmt.Sprintf("%s/%s", pathType, ip.Path.Path))
}

func ToHTTPRouteMatch(routePath networkingv1.HTTPIngressPath, path *field.Path, toImplementationSpecificPathMatch i2gw.ImplementationSpecificHTTPPathTypeMatchConverter) (*gatewayv1.HTTPRouteMatch, *field.Error) {
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
