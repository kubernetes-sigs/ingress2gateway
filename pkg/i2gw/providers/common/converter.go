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
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ToGateway converts the received ingresses to i2gw.GatewayResources,
// without taking into consideration any provider specific logic.
func ToGateway(ingresses []networkingv1.Ingress, options i2gw.ProviderImplementationSpecificOptions) (i2gw.GatewayResources, field.ErrorList) {
	aggregator := ingressAggregator{ruleGroups: map[ruleGroupKey]*ingressRuleGroup{}}

	var errs field.ErrorList
	for _, ingress := range ingresses {
		aggregator.addIngress(ingress)
	}
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	routes, gateways, errs := aggregator.toHTTPRoutesAndGateways(options)
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	routeByKey := make(map[types.NamespacedName]gatewayv1.HTTPRoute)
	for _, route := range routes {
		key := types.NamespacedName{Namespace: route.Namespace, Name: route.Name}
		routeByKey[key] = route
	}

	gatewayByKey := make(map[types.NamespacedName]gatewayv1.Gateway)
	for _, gateway := range gateways {
		key := types.NamespacedName{Namespace: gateway.Namespace, Name: gateway.Name}
		gatewayByKey[key] = gateway
	}

	return i2gw.GatewayResources{
		Gateways:   gatewayByKey,
		HTTPRoutes: routeByKey,
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
}

type pathMatchKey string

type ingressRuleGroup struct {
	namespace    string
	name         string
	ingressClass string
	host         string
	tls          []networkingv1.IngressTLS
	rules        []ingressRule
}

type ingressRule struct {
	rule networkingv1.IngressRule
}

type ingressDefaultBackend struct {
	name         string
	namespace    string
	ingressClass string
	backend      networkingv1.IngressBackend
}

type ingressPath struct {
	ruleIdx  int
	pathIdx  int
	ruleType string
	path     networkingv1.HTTPIngressPath
}

func (a *ingressAggregator) addIngress(ingress networkingv1.Ingress) {
	var ingressClass string
	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName != "" {
		ingressClass = *ingress.Spec.IngressClassName
	} else if _, ok := ingress.Annotations[networkingv1beta1.AnnotationIngressClass]; ok {
		ingressClass = ingress.Annotations[networkingv1beta1.AnnotationIngressClass]
	} else {
		ingressClass = ingress.Name
	}
	for _, rule := range ingress.Spec.Rules {
		a.addIngressRule(ingress.Namespace, ingress.Name, ingressClass, rule, ingress.Spec)
	}
	if ingress.Spec.DefaultBackend != nil {
		a.defaultBackends = append(a.defaultBackends, ingressDefaultBackend{
			name:         ingress.Name,
			namespace:    ingress.Namespace,
			ingressClass: ingressClass,
			backend:      *ingress.Spec.DefaultBackend,
		})
	}
}

func (a *ingressAggregator) addIngressRule(namespace, name, ingressClass string, rule networkingv1.IngressRule, iSpec networkingv1.IngressSpec) {
	rgKey := ruleGroupKey(fmt.Sprintf("%s/%s/%s", namespace, ingressClass, rule.Host))
	rg, ok := a.ruleGroups[rgKey]
	if !ok {
		rg = &ingressRuleGroup{
			namespace:    namespace,
			name:         name,
			ingressClass: ingressClass,
			host:         rule.Host,
		}
		a.ruleGroups[rgKey] = rg
	}
	if len(iSpec.TLS) > 0 {
		rg.tls = append(rg.tls, iSpec.TLS...)
	}
	rg.rules = append(rg.rules, ingressRule{rule: rule})
}

func (a *ingressAggregator) toHTTPRoutesAndGateways(options i2gw.ProviderImplementationSpecificOptions) ([]gatewayv1.HTTPRoute, []gatewayv1.Gateway, field.ErrorList) {
	var httpRoutes []gatewayv1.HTTPRoute
	var errors field.ErrorList
	listenersByNamespacedGateway := map[string][]gatewayv1.Listener{}

	for _, rg := range a.ruleGroups {
		listener := gatewayv1.Listener{}
		if rg.host != "" {
			listener.Hostname = (*gatewayv1.Hostname)(&rg.host)
		} else if len(rg.tls) == 1 && len(rg.tls[0].Hosts) == 1 {
			listener.Hostname = (*gatewayv1.Hostname)(&rg.tls[0].Hosts[0])
		}
		if len(rg.tls) > 0 {
			listener.TLS = &gatewayv1.GatewayTLSConfig{}
		}
		for _, tls := range rg.tls {
			listener.TLS.CertificateRefs = append(listener.TLS.CertificateRefs,
				gatewayv1.SecretObjectReference{Name: gatewayv1.ObjectName(tls.SecretName)})
		}
		gwKey := fmt.Sprintf("%s/%s", rg.namespace, rg.ingressClass)
		listenersByNamespacedGateway[gwKey] = append(listenersByNamespacedGateway[gwKey], listener)
		httpRoute, errs := rg.toHTTPRoute(options)
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

		backendRef, err := toBackendRef(db.backend, field.NewPath(db.name, "paths", "backends").Index(i))
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

func (rg *ingressRuleGroup) toHTTPRoute(options i2gw.ProviderImplementationSpecificOptions) (gatewayv1.HTTPRoute, field.ErrorList) {
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

		backendRefs, errs := rg.configureBackendRef(paths)
		errors = append(errors, errs...)
		hrRule.BackendRefs = backendRefs

		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, hrRule)
	}

	return httpRoute, errors
}

func (rg *ingressRuleGroup) configureBackendRef(paths []ingressPath) ([]gatewayv1.HTTPBackendRef, field.ErrorList) {
	var errors field.ErrorList
	var backendRefs []gatewayv1.HTTPBackendRef

	for i, path := range paths {
		backendRef, err := toBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		backendRefs = append(backendRefs, gatewayv1.HTTPBackendRef{BackendRef: *backendRef})
	}

	return removeBackendRefsDuplicates(backendRefs), errors
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

func toBackendRef(ib networkingv1.IngressBackend, path *field.Path) (*gatewayv1.BackendRef, *field.Error) {
	if ib.Service != nil {
		if ib.Service.Port.Name != "" {
			fieldPath := path.Child("service", "port")
			return nil, field.Invalid(fieldPath, "name", fmt.Sprintf("named ports not supported: %s", ib.Service.Port.Name))
		}
		return &gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Name: gatewayv1.ObjectName(ib.Service.Name),
				Port: (*gatewayv1.PortNumber)(&ib.Service.Port.Number),
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
