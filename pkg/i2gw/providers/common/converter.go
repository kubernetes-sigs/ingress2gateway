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
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ToHTTPRouteMatchOption is an option to give the providers the possibility to customize
// HTTPRouteMatch support.
type ToHTTPRouteMatchOption func(routePath networkingv1.HTTPIngressPath, path *field.Path) (*gatewayv1beta1.HTTPRouteMatch, *field.Error)

// ToGateway converts the received ingresses to i2gw.GatewayResources,
// without taking into consideration any provider specific logic.
func ToGateway(ingresses []networkingv1.Ingress, toHTTPRouteMatchCustom ToHTTPRouteMatchOption) (i2gw.GatewayResources, field.ErrorList) {
	aggregator := ingressAggregator{ruleGroups: map[ruleGroupKey]*ingressRuleGroup{}}

	var errs field.ErrorList
	for _, ingress := range ingresses {
		errs = append(errs, aggregator.addIngress(ingress)...)
	}
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	routes, gateways, errs := aggregator.toHTTPRoutesAndGateways(toHTTPRouteMatchCustom)
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	routeByKey := make(map[types.NamespacedName]gatewayv1beta1.HTTPRoute)
	for _, route := range routes {
		key := types.NamespacedName{Namespace: route.Namespace, Name: route.Name}
		routeByKey[key] = route
	}

	gatewayByKey := make(map[types.NamespacedName]gatewayv1beta1.Gateway)
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
		Version: "v1beta1",
		Kind:    "Gateway",
	}

	HTTPRouteGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1beta1",
		Kind:    "HTTPRoute",
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

func (a *ingressAggregator) addIngress(ingress networkingv1.Ingress) field.ErrorList {
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
	return nil
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

func (a *ingressAggregator) toHTTPRoutesAndGateways(toHTTPRouteMatchCustom ToHTTPRouteMatchOption) ([]gatewayv1beta1.HTTPRoute, []gatewayv1beta1.Gateway, field.ErrorList) {
	var httpRoutes []gatewayv1beta1.HTTPRoute
	var errors field.ErrorList
	listenersByNamespacedGateway := map[string][]gatewayv1beta1.Listener{}

	for _, rg := range a.ruleGroups {
		listener := gatewayv1beta1.Listener{}
		if rg.host != "" {
			listener.Hostname = (*gatewayv1beta1.Hostname)(&rg.host)
		} else if len(rg.tls) == 1 && len(rg.tls[0].Hosts) == 1 {
			listener.Hostname = (*gatewayv1beta1.Hostname)(&rg.tls[0].Hosts[0])
		}
		if len(rg.tls) > 0 {
			listener.TLS = &gatewayv1beta1.GatewayTLSConfig{}
		}
		for _, tls := range rg.tls {
			listener.TLS.CertificateRefs = append(listener.TLS.CertificateRefs,
				gatewayv1beta1.SecretObjectReference{Name: gatewayv1beta1.ObjectName(tls.SecretName)})
		}
		gwKey := fmt.Sprintf("%s/%s", rg.namespace, rg.ingressClass)
		listenersByNamespacedGateway[gwKey] = append(listenersByNamespacedGateway[gwKey], listener)
		httpRoute, errs := rg.toHTTPRoute(toHTTPRouteMatchCustom)
		httpRoutes = append(httpRoutes, httpRoute)
		errors = append(errors, errs...)
	}

	for i, db := range a.defaultBackends {
		httpRoute := gatewayv1beta1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-default-backend", db.name),
				Namespace: db.namespace,
			},
			Spec: gatewayv1beta1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
					ParentRefs: []gatewayv1beta1.ParentReference{{
						Name: gatewayv1beta1.ObjectName(db.ingressClass),
					}},
				},
			},
			Status: gatewayv1beta1.HTTPRouteStatus{
				RouteStatus: gatewayv1beta1.RouteStatus{
					Parents: []gatewayv1beta1.RouteParentStatus{},
				},
			},
		}
		httpRoute.SetGroupVersionKind(HTTPRouteGVK)

		backendRef, err := toBackendRef(db.backend, field.NewPath(db.name, "paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
		} else {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1beta1.HTTPRouteRule{
				BackendRefs: []gatewayv1beta1.HTTPBackendRef{{BackendRef: *backendRef}},
			})
		}

		httpRoutes = append(httpRoutes, httpRoute)
	}

	gatewaysByKey := map[string]*gatewayv1beta1.Gateway{}
	for gwKey, listeners := range listenersByNamespacedGateway {
		parts := strings.Split(gwKey, "/")
		if len(parts) != 2 {
			errors = append(errors, field.Invalid(field.NewPath(""), "", fmt.Sprintf("error generating Gateway listeners for key: %s", gwKey)))
			continue
		}
		gateway := gatewaysByKey[gwKey]
		if gateway == nil {
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: parts[0],
					Name:      parts[1],
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: gatewayv1beta1.ObjectName(parts[1]),
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

			gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1beta1.Listener{
				Name:     gatewayv1beta1.SectionName(fmt.Sprintf("%shttp", listenerNamePrefix)),
				Hostname: listener.Hostname,
				Port:     80,
				Protocol: gatewayv1beta1.HTTPProtocolType,
			})
			if listener.TLS != nil {
				gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1beta1.Listener{
					Name:     gatewayv1beta1.SectionName(fmt.Sprintf("%shttps", listenerNamePrefix)),
					Hostname: listener.Hostname,
					Port:     443,
					Protocol: gatewayv1beta1.HTTPSProtocolType,
					TLS:      listener.TLS,
				})
			}
		}
	}

	var gateways []gatewayv1beta1.Gateway
	for _, gw := range gatewaysByKey {
		gateways = append(gateways, *gw)
	}

	return httpRoutes, gateways, errors
}

func (rg *ingressRuleGroup) toHTTPRoute(toHTTPRouteMatchCustom ToHTTPRouteMatchOption) (gatewayv1beta1.HTTPRoute, field.ErrorList) {
	pathsByMatchGroup := map[pathMatchKey][]ingressPath{}
	var errors field.ErrorList

	for i, ir := range rg.rules {
		for j, path := range ir.rule.HTTP.Paths {
			ip := ingressPath{ruleIdx: i, pathIdx: j, ruleType: "http", path: path}
			pmKey := getPathMatchKey(ip)
			pathsByMatchGroup[pmKey] = append(pathsByMatchGroup[pmKey], ip)
		}
	}

	httpRoute := gatewayv1beta1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RouteName(rg.name, rg.host),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1beta1.HTTPRouteSpec{},
		Status: gatewayv1beta1.HTTPRouteStatus{
			RouteStatus: gatewayv1beta1.RouteStatus{
				Parents: []gatewayv1beta1.RouteParentStatus{},
			},
		},
	}
	httpRoute.SetGroupVersionKind(HTTPRouteGVK)

	if rg.ingressClass != "" {
		httpRoute.Spec.ParentRefs = []gatewayv1beta1.ParentReference{{Name: gatewayv1beta1.ObjectName(rg.ingressClass)}}
	}
	if rg.host != "" {
		httpRoute.Spec.Hostnames = []gatewayv1beta1.Hostname{gatewayv1beta1.Hostname(rg.host)}
	}

	for _, paths := range pathsByMatchGroup {
		path := paths[0]
		fieldPath := field.NewPath("spec", "rules").Index(path.ruleIdx).Child(path.ruleType).Child("paths").Index(path.pathIdx)
		var match *gatewayv1beta1.HTTPRouteMatch
		var err *field.Error
		if toHTTPRouteMatchCustom != nil {
			match, err = toHTTPRouteMatchCustom(path.path, fieldPath)
		} else {
			match, err = toHTTPRouteMatch(path.path, fieldPath)
		}
		if err != nil {
			errors = append(errors, err)
			continue
		}
		hrRule := gatewayv1beta1.HTTPRouteRule{
			Matches: []gatewayv1beta1.HTTPRouteMatch{*match},
		}

		backendRefs, errs := rg.configureBackendRef(paths)
		errors = append(errors, errs...)
		hrRule.BackendRefs = backendRefs

		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, hrRule)
	}

	return httpRoute, errors
}

func (rg *ingressRuleGroup) configureBackendRef(paths []ingressPath) ([]gatewayv1beta1.HTTPBackendRef, field.ErrorList) {
	var errors field.ErrorList
	var backendRefs []gatewayv1beta1.HTTPBackendRef

	for i, path := range paths {
		backendRef, err := toBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		backendRefs = append(backendRefs, gatewayv1beta1.HTTPBackendRef{BackendRef: *backendRef})
	}

	return backendRefs, errors
}

func getPathMatchKey(ip ingressPath) pathMatchKey {
	var pathType string
	if ip.path.PathType != nil {
		pathType = string(*ip.path.PathType)
	}
	return pathMatchKey(fmt.Sprintf("%s/%s", pathType, ip.path.Path))
}

func toHTTPRouteMatch(routePath networkingv1.HTTPIngressPath, path *field.Path) (*gatewayv1beta1.HTTPRouteMatch, *field.Error) {
	pmPrefix := gatewayv1beta1.PathMatchPathPrefix
	pmExact := gatewayv1beta1.PathMatchExact

	match := &gatewayv1beta1.HTTPRouteMatch{Path: &gatewayv1beta1.HTTPPathMatch{Value: &routePath.Path}}
	//exhaustive:ignore -explicit-exhaustive-switch
	// networkingv1.PathTypeImplementationSpecific is not supported here, hence it goes into default case.
	switch *routePath.PathType {
	case networkingv1.PathTypePrefix:
		match.Path.Type = &pmPrefix
	case networkingv1.PathTypeExact:
		match.Path.Type = &pmExact
	default:
		return nil, field.Invalid(path.Child("pathType"), routePath.PathType, fmt.Sprintf("unsupported path match type: %s", *routePath.PathType))
	}

	return match, nil
}

func toBackendRef(ib networkingv1.IngressBackend, path *field.Path) (*gatewayv1beta1.BackendRef, *field.Error) {
	if ib.Service != nil {
		if ib.Service.Port.Name != "" {
			fieldPath := path.Child("service", "port")
			return nil, field.Invalid(fieldPath, "name", fmt.Sprintf("named ports not supported: %s", ib.Service.Port.Name))
		}
		return &gatewayv1beta1.BackendRef{
			BackendObjectReference: gatewayv1beta1.BackendObjectReference{
				Name: gatewayv1beta1.ObjectName(ib.Service.Name),
				Port: (*gatewayv1beta1.PortNumber)(&ib.Service.Port.Number),
			},
		}, nil
	}
	return &gatewayv1beta1.BackendRef{
		BackendObjectReference: gatewayv1beta1.BackendObjectReference{
			Group: (*gatewayv1beta1.Group)(ib.Resource.APIGroup),
			Kind:  (*gatewayv1beta1.Kind)(&ib.Resource.Kind),
			Name:  gatewayv1beta1.ObjectName(ib.Resource.Name),
		},
	}, nil
}
