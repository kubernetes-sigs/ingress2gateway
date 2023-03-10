/*
Copyright © 2023 Kubernetes Authors

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
package translator

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	gatewayGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1beta1",
		Kind:    "Gateway",
	}

	httpRouteGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1beta1",
		Kind:    "HTTPRoute",
	}
)

type ruleKeyGroup struct {
	ruleGroup    *ingressRuleGroup
	ruleGroupKey string
}

type IngressAggregator struct {
	ruleGroups      []ruleKeyGroup
	defaultBackends []ingressDefaultBackend
}

func findRuleGroupWithKey(key string, ruleGroups []ruleKeyGroup) (*ruleKeyGroup, int) {
	for index, g := range ruleGroups {
		if g.ruleGroupKey == key {
			return &g, index
		}
	}

	return nil, 0
}

func NewAggregator() *IngressAggregator {
	return &IngressAggregator{
		ruleGroups:      make([]ruleKeyGroup, 0),
		defaultBackends: make([]ingressDefaultBackend, 0),
	}
}

type pathMatchKey string

type ingressRuleGroup struct {
	namespace    string
	ingressClass string
	host         string
	tls          []networkingv1.IngressTLS
	rules        []ingressRule
}

type ingressRule struct {
	rule        networkingv1.IngressRule
	annotations *annotations
}

type ingressDefaultBackend struct {
	name         string
	namespace    string
	ingressClass string
	backend      networkingv1.IngressBackend
}

type ingressPath struct {
	path        networkingv1.HTTPIngressPath
	annotations *annotations
}

func (a *IngressAggregator) addIngress(provider IngressProvider, ingress networkingv1.Ingress) {
	var ingressClass string
	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName != "" {
		ingressClass = *ingress.Spec.IngressClassName
	} else if _, ok := ingress.Annotations[networkingv1beta1.AnnotationIngressClass]; ok {
		ingressClass = ingress.Annotations[networkingv1beta1.AnnotationIngressClass]
	} else {
		ingressClass = ingress.Name
	}
	anno := retrieveAnnotations(provider, ingress)
	for _, rule := range ingress.Spec.Rules {
		a.addIngressRule(ingress.Namespace, ingressClass, rule, ingress.Spec, anno)
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

type ListenersWithKey struct {
	Key       string
	Listeners []gatewayv1beta1.Listener
}

func findListenersByKey(key string, listenrs []ListenersWithKey) (*ListenersWithKey, int) {
	for index, listener := range listenrs {
		if listener.Key == key {
			return &listener, index
		}
	}

	return nil, 0
}

type GatewayWithKey struct {
	Key     string
	Gateway gatewayv1beta1.Gateway
}

func findGatewayByKey(key string, gateways []GatewayWithKey) *GatewayWithKey {
	for _, gateway := range gateways {
		if gateway.Key == key {
			return &gateway
		}
	}

	return nil
}

func (a *IngressAggregator) addIngressRule(namespace, ingressClass string, rule networkingv1.IngressRule, iSpec networkingv1.IngressSpec, anno *annotations) {
	rgKey := fmt.Sprintf("%s/%s/%s", namespace, ingressClass, rule.Host)
	rg, _ := findRuleGroupWithKey(rgKey, a.ruleGroups)
	needCreate := false
	if rg == nil {
		needCreate = true
		irg := &ingressRuleGroup{
			namespace:    namespace,
			ingressClass: ingressClass,
			host:         rule.Host,
		}

		rg = &ruleKeyGroup{
			ruleGroup:    irg,
			ruleGroupKey: rgKey,
		}
	}

	if len(iSpec.TLS) > 0 {
		rg.ruleGroup.tls = append(rg.ruleGroup.tls, iSpec.TLS...)
	}

	rg.ruleGroup.rules = append(rg.ruleGroup.rules, ingressRule{rule: rule, annotations: anno})

	if needCreate {
		a.ruleGroups = append(a.ruleGroups, *rg)
	}
}

func (a *IngressAggregator) convert() (ResultResources, []error) {
	result := NewResultResources()
	var errs []error

	listenersByNamespacedGateway := []ListenersWithKey{}
	for _, rg := range a.ruleGroups {
		listener := gatewayv1beta1.Listener{}
		if rg.ruleGroup.host != "" {
			listener.Hostname = (*gatewayv1beta1.Hostname)(&rg.ruleGroup.host)
		} else if len(rg.ruleGroup.tls) == 1 && len(rg.ruleGroup.tls[0].Hosts) == 1 {
			listener.Hostname = (*gatewayv1beta1.Hostname)(&rg.ruleGroup.tls[0].Hosts[0])
		}
		if len(rg.ruleGroup.tls) > 0 {
			listener.TLS = &gatewayv1beta1.GatewayTLSConfig{}
		}
		for _, tls := range rg.ruleGroup.tls {
			listener.TLS.CertificateRefs = append(listener.TLS.CertificateRefs,
				gatewayv1beta1.SecretObjectReference{Name: gatewayv1beta1.ObjectName(tls.SecretName)})
		}
		gwKey := fmt.Sprintf("%s/%s", rg.ruleGroup.namespace, rg.ruleGroup.ingressClass)
		l, index := findListenersByKey(gwKey, listenersByNamespacedGateway)
		if l == nil {
			listenersByNamespacedGateway = append(listenersByNamespacedGateway, ListenersWithKey{
				Key:       gwKey,
				Listeners: []gatewayv1beta1.Listener{listener},
			})
		} else {
			listenersByNamespacedGateway[index].Listeners = append(listenersByNamespacedGateway[index].Listeners, listener)
		}

		httpRoute, errors := rg.ruleGroup.toHTTPRoute()
		result.HTTPRoutes = append(result.HTTPRoutes, httpRoute)
		errs = append(errs, errors...)
	}

	for _, db := range a.defaultBackends {
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
		httpRoute.SetGroupVersionKind(httpRouteGVK)

		backendRef, err := toBackendRef(db.backend)
		if err != nil {
			errs = append(errs, err)
		} else {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1beta1.HTTPRouteRule{
				BackendRefs: []gatewayv1beta1.HTTPBackendRef{{BackendRef: *backendRef}},
			})
		}

		result.HTTPRoutes = append(result.HTTPRoutes, httpRoute)
	}

	gatewaysByKey := []GatewayWithKey{}
	for _, listeners := range listenersByNamespacedGateway {
		parts := strings.Split(listeners.Key, "/")
		if len(parts) != 2 {
			errs = append(errs, fmt.Errorf("error generating Gateway listeners for key: %s", listeners.Key))
			continue
		}

		gatewaybyKey := findGatewayByKey(listeners.Key, gatewaysByKey)
		needCreate := false
		if gatewaybyKey == nil {
			needCreate = true

			gateway := &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: parts[0],
					Name:      parts[1],
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: gatewayv1beta1.ObjectName(parts[1]),
				},
			}
			gateway.SetGroupVersionKind(gatewayGVK)

			gatewaybyKey = &GatewayWithKey{
				Gateway: *gateway,
				Key:     listeners.Key,
			}
		}
		for _, listener := range listeners.Listeners {
			var listenerNamePrefix string
			if listener.Hostname != nil && *listener.Hostname != "" {
				listenerNamePrefix = nameFromHost(string(*listener.Hostname))
			}

			gatewaybyKey.Gateway.Spec.Listeners = append(gatewaybyKey.Gateway.Spec.Listeners, gatewayv1beta1.Listener{
				Name:     gatewayv1beta1.SectionName(fmt.Sprintf("%s-http", listenerNamePrefix)),
				Hostname: listener.Hostname,
				Port:     80,
				Protocol: gatewayv1beta1.HTTPProtocolType,
			})
			if listener.TLS != nil {
				gatewaybyKey.Gateway.Spec.Listeners = append(gatewaybyKey.Gateway.Spec.Listeners, gatewayv1beta1.Listener{
					Name:     gatewayv1beta1.SectionName(fmt.Sprintf("%s-https", listenerNamePrefix)),
					Hostname: listener.Hostname,
					Port:     443,
					Protocol: gatewayv1beta1.HTTPSProtocolType,
					TLS:      listener.TLS,
				})
			}
		}

		if needCreate {
			gatewaysByKey = append(gatewaysByKey, *gatewaybyKey)
		}
	}

	for _, gw := range gatewaysByKey {
		result.Gateways = append(result.Gateways, gw.Gateway)
	}

	return *result, errs
}

type PathsWithKey struct {
	PathMatchKey string
	Paths        []ingressPath
}

func findPathWithKey(key string, paths []PathsWithKey) (*PathsWithKey, int) {
	for index, path := range paths {
		if path.PathMatchKey == key {
			return &path, index
		}
	}
	return nil, 0
}

func (rg *ingressRuleGroup) toHTTPRoute() (gatewayv1beta1.HTTPRoute, []error) {
	pathsByMatchGroup := []PathsWithKey{}
	errors := []error{}

	for _, ir := range rg.rules {
		for _, path := range ir.rule.HTTP.Paths {
			ip := ingressPath{path: path, annotations: ir.annotations}
			pmKey := getPathMatchKey(ip)
			path, index := findPathWithKey(string(pmKey), pathsByMatchGroup)
			if path == nil {
				pathsByMatchGroup = append(pathsByMatchGroup, PathsWithKey{
					PathMatchKey: string(pmKey),
					Paths:        []ingressPath{ip},
				})
			} else {
				pathsByMatchGroup[index].Paths = append(pathsByMatchGroup[index].Paths, ip)
			}
		}
	}

	httpRoute := gatewayv1beta1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameFromHost(rg.host),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1beta1.HTTPRouteSpec{},
		Status: gatewayv1beta1.HTTPRouteStatus{
			RouteStatus: gatewayv1beta1.RouteStatus{
				Parents: []gatewayv1beta1.RouteParentStatus{},
			},
		},
	}
	httpRoute.SetGroupVersionKind(httpRouteGVK)

	if rg.ingressClass != "" {
		httpRoute.Spec.ParentRefs = []gatewayv1beta1.ParentReference{{Name: gatewayv1beta1.ObjectName(rg.ingressClass)}}
	}
	if rg.host != "" {
		httpRoute.Spec.Hostnames = []gatewayv1beta1.Hostname{gatewayv1beta1.Hostname(rg.host)}
	}

	for _, paths := range pathsByMatchGroup {
		match, err := toHTTPRouteMatch(paths.Paths[0])
		if err != nil {
			errors = append(errors, err)
			continue
		}
		hrRule := gatewayv1beta1.HTTPRouteRule{
			Matches: []gatewayv1beta1.HTTPRouteMatch{*match},
		}

		var numWeightedBackends, totalWeightSet int32
		for _, path := range paths.Paths {
			backendRef, err := toBackendRef(path.path.Backend)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			if path.annotations != nil && path.annotations.canary != nil && path.annotations.canary.weight != 0 {
				weight := int32(path.annotations.canary.weight)
				backendRef.Weight = &weight
				totalWeightSet += weight
				numWeightedBackends++
			}
			hrRule.BackendRefs = append(hrRule.BackendRefs, gatewayv1beta1.HTTPBackendRef{BackendRef: *backendRef})
		}
		if numWeightedBackends > 0 && numWeightedBackends < int32(len(hrRule.BackendRefs)) {
			weightToSet := (int32(100) - totalWeightSet) / (int32(len(hrRule.BackendRefs)) - numWeightedBackends)
			for i, br := range hrRule.BackendRefs {
				if br.Weight == nil {
					br.Weight = &weightToSet
					hrRule.BackendRefs[i] = br
				}
			}
		}
		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, hrRule)
	}

	return httpRoute, errors
}

func getPathMatchKey(ip ingressPath) pathMatchKey {
	var pathType string
	if ip.path.PathType != nil {
		pathType = string(*ip.path.PathType)
	}
	var canaryHeaderKey string
	if ip.annotations != nil && ip.annotations.canary != nil && ip.annotations.canary.headerKey != "" {
		canaryHeaderKey = ip.annotations.canary.headerKey
	}
	return pathMatchKey(fmt.Sprintf("%s/%s/%s", pathType, ip.path.Path, canaryHeaderKey))
}

func toHTTPRouteMatch(ip ingressPath) (*gatewayv1beta1.HTTPRouteMatch, error) {
	pmPrefix := gatewayv1beta1.PathMatchPathPrefix
	pmExact := gatewayv1beta1.PathMatchExact
	hmExact := gatewayv1beta1.HeaderMatchExact
	hmRegex := gatewayv1beta1.HeaderMatchRegularExpression

	match := &gatewayv1beta1.HTTPRouteMatch{Path: &gatewayv1beta1.HTTPPathMatch{Value: &ip.path.Path}}
	switch *ip.path.PathType {
	case networkingv1.PathTypePrefix:
		match.Path.Type = &pmPrefix
	case networkingv1.PathTypeExact:
		match.Path.Type = &pmExact
	default:
		return nil, fmt.Errorf("unsupported path match type: %s", *ip.path.PathType)
	}

	if ip.annotations != nil && ip.annotations.canary != nil && ip.annotations.canary.headerKey != "" {
		headerMatch := gatewayv1beta1.HTTPHeaderMatch{
			Name:  gatewayv1beta1.HTTPHeaderName(ip.annotations.canary.headerKey),
			Value: ip.annotations.canary.headerValue,
			Type:  &hmExact,
		}
		if ip.annotations.canary.headerRegexMatch {
			headerMatch.Type = &hmRegex
		}
		match.Headers = []gatewayv1beta1.HTTPHeaderMatch{headerMatch}
	}

	return match, nil
}

func toBackendRef(ib networkingv1.IngressBackend) (*gatewayv1beta1.BackendRef, error) {
	if ib.Service != nil {
		if ib.Service.Port.Name != "" {
			return nil, fmt.Errorf("named ports not supported: %s", ib.Service.Port.Name)
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
