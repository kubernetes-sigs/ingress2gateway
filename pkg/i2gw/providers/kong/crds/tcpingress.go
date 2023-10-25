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

package crds

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kongv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
)

// TCPIngressToGatewayAPI converts the received TCPingresses to i2gw.GatewayResources,
func TCPIngressToGatewayAPI(ingresses []kongv1beta1.TCPIngress) (i2gw.GatewayResources, field.ErrorList) {
	aggregator := tcpIngressAggregator{ruleGroups: map[ruleGroupKey]*tcpIngressRuleGroup{}}

	var errs field.ErrorList
	for _, ingress := range ingresses {
		aggregator.addIngress(ingress)
	}
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	tcpRoutes, tlsRoutes, gateways, errs := aggregator.toRoutesAndGateways()
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	tcpRouteByKey := make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute)
	for _, route := range tcpRoutes {
		key := types.NamespacedName{Namespace: route.Namespace, Name: route.Name}
		tcpRouteByKey[key] = route
	}

	tlsRouteByKey := make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute)
	for _, route := range tlsRoutes {
		key := types.NamespacedName{Namespace: route.Namespace, Name: route.Name}
		tlsRouteByKey[key] = route
	}

	gatewayByKey := make(map[types.NamespacedName]gatewayv1.Gateway)
	for _, gateway := range gateways {
		key := types.NamespacedName{Namespace: gateway.Namespace, Name: gateway.Name}
		gatewayByKey[key] = gateway
	}

	return i2gw.GatewayResources{
		Gateways:  gatewayByKey,
		TCPRoutes: tcpRouteByKey,
		TLSRoutes: tlsRouteByKey,
	}, nil
}

func (a *tcpIngressAggregator) addIngress(tcpIngress kongv1beta1.TCPIngress) {
	var ingressClass string
	if _, ok := tcpIngress.Annotations[networkingv1beta1.AnnotationIngressClass]; ok {
		ingressClass = tcpIngress.Annotations[networkingv1beta1.AnnotationIngressClass]
	} else {
		ingressClass = tcpIngress.Name
	}
	for _, rule := range tcpIngress.Spec.Rules {
		a.addIngressRule(tcpIngress.Namespace, tcpIngress.Name, ingressClass, rule, tcpIngress.Spec)
	}
}

func (a *tcpIngressAggregator) addIngressRule(namespace, name, ingressClass string, rule kongv1beta1.IngressRule, iSpec kongv1beta1.TCPIngressSpec) {
	rgKey := ruleGroupKey(fmt.Sprintf("%s/%s/%s/%d", namespace, ingressClass, rule.Host, rule.Port))
	rg, ok := a.ruleGroups[rgKey]
	if !ok {
		rg = &tcpIngressRuleGroup{
			namespace:    namespace,
			name:         name,
			ingressClass: ingressClass,
			host:         rule.Host,
			port:         rule.Port,
		}
		a.ruleGroups[rgKey] = rg
	}
	if len(iSpec.TLS) > 0 {
		rg.tls = append(rg.tls, iSpec.TLS...)
	}
	rg.rules = append(rg.rules, ingressRule{rule: rule})
}

func (a *tcpIngressAggregator) toRoutesAndGateways() ([]gatewayv1alpha2.TCPRoute, []gatewayv1alpha2.TLSRoute, []gatewayv1.Gateway, field.ErrorList) {
	var tcpRoutes []gatewayv1alpha2.TCPRoute
	var tlsRoutes []gatewayv1alpha2.TLSRoute

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
			listener.TLS = &gatewayv1.GatewayTLSConfig{
				Mode: common.PtrTo(gatewayv1.TLSModePassthrough),
			}
		}
		listener.Port = gatewayv1.PortNumber(rg.port)
		for _, tls := range rg.tls {
			listener.TLS.CertificateRefs = append(listener.TLS.CertificateRefs,
				gatewayv1.SecretObjectReference{
					Group: common.PtrTo(gatewayv1.Group("")),
					Kind:  common.PtrTo(gatewayv1.Kind("Secret")),
					Name:  gatewayv1.ObjectName(tls.SecretName),
				})
		}
		gwKey := fmt.Sprintf("%s/%s", rg.namespace, rg.ingressClass)
		listenersByNamespacedGateway[gwKey] = append(listenersByNamespacedGateway[gwKey], listener)
		var errs field.ErrorList
		if listener.TLS == nil {
			tcpRoutes = append(tcpRoutes, rg.toTCPRoute())
		} else {
			tlsRoutes = append(tlsRoutes, rg.toTLSRoute())
		}
		errors = append(errors, errs...)
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
			gateway.SetGroupVersionKind(common.GatewayGVK)
			gatewaysByKey[gwKey] = gateway
		}
		for _, listener := range listeners {
			hostname := ""
			if listener.Hostname != nil {
				hostname = string(*listener.Hostname)
			}
			if listener.TLS != nil {
				gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1.Listener{
					Hostname: listener.Hostname,
					Protocol: gatewayv1.TLSProtocolType,
					Port:     listener.Port,
					Name:     *buildSectionName("tls", common.NameFromHost(hostname), strconv.Itoa(int(listener.Port))),
					TLS:      listener.TLS,
				})
			} else {
				gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1.Listener{
					Hostname: listener.Hostname,
					Protocol: gatewayv1.TCPProtocolType,
					Port:     listener.Port,
					Name:     *buildSectionName("tcp", common.NameFromHost(hostname), strconv.Itoa(int(listener.Port))),
				})
			}
		}
	}

	var gateways []gatewayv1.Gateway
	for _, gw := range gatewaysByKey {
		gateways = append(gateways, *gw)
	}

	return tcpRoutes, tlsRoutes, gateways, errors
}

func (rg *tcpIngressRuleGroup) toTCPRoute() gatewayv1alpha2.TCPRoute {
	tcpRoute := gatewayv1alpha2.TCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RouteName(rg.name, rg.host),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1alpha2.TCPRouteSpec{},
		Status: gatewayv1alpha2.TCPRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{},
			},
		},
	}
	tcpRoute.SetGroupVersionKind(common.TCPRouteGVK)

	if rg.ingressClass != "" {
		tcpRoute.Spec.ParentRefs = []gatewayv1.ParentReference{
			{
				Name:        gatewayv1.ObjectName(rg.ingressClass),
				SectionName: buildSectionName("tcp", common.NameFromHost(rg.host), strconv.Itoa(rg.port)),
			},
		}
	}

	for _, rule := range rg.rules {
		tcpRoute.Spec.Rules = append(tcpRoute.Spec.Rules,
			gatewayv1alpha2.TCPRouteRule{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(rule.rule.Backend.ServiceName),
							Port: common.PtrTo(gatewayv1.PortNumber(rule.rule.Backend.ServicePort)),
						},
					},
				},
			},
		)
	}

	return tcpRoute
}

func (rg *tcpIngressRuleGroup) toTLSRoute() gatewayv1alpha2.TLSRoute {
	tlsRoute := gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RouteName(rg.name, rg.host),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1alpha2.TLSRouteSpec{},
		Status: gatewayv1alpha2.TLSRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{},
			},
		},
	}
	tlsRoute.SetGroupVersionKind(common.TLSRouteGVK)

	if rg.ingressClass != "" {
		tlsRoute.Spec.ParentRefs = []gatewayv1.ParentReference{
			{
				Name:        gatewayv1.ObjectName(rg.ingressClass),
				SectionName: buildSectionName("tls", common.NameFromHost(rg.host), strconv.Itoa(rg.port)),
			},
		}
	}

	for _, rule := range rg.rules {
		tlsRoute.Spec.Rules = append(tlsRoute.Spec.Rules,
			gatewayv1alpha2.TLSRouteRule{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(rule.rule.Backend.ServiceName),
							Port: common.PtrTo(gatewayv1.PortNumber(rule.rule.Backend.ServicePort)),
						},
					},
				},
			},
		)
	}

	return tlsRoute
}
