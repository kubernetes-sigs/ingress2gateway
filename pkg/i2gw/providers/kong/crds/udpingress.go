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

	kongv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func UDPIngressToGatewayAPI(ingresses []kongv1beta1.UDPIngress) (i2gw.GatewayResources, field.ErrorList) {
	aggregator := udpIngressAggregator{ruleGroups: map[ruleGroupKey]*udpIngressRuleGroup{}}

	var errs field.ErrorList
	for _, ingress := range ingresses {
		errs = append(errs, aggregator.addIngress(ingress)...)
	}
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	udpRoutes, gateways, errs := aggregator.toRoutesAndGateways()
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	udpRouteByKey := make(map[types.NamespacedName]gatewayv1alpha2.UDPRoute)
	for _, route := range udpRoutes {
		key := types.NamespacedName{Namespace: route.Namespace, Name: route.Name}
		udpRouteByKey[key] = route
	}

	gatewayByKey := make(map[types.NamespacedName]gatewayv1beta1.Gateway)
	for _, gateway := range gateways {
		key := types.NamespacedName{Namespace: gateway.Namespace, Name: gateway.Name}
		gatewayByKey[key] = gateway
	}

	return i2gw.GatewayResources{
		Gateways:  gatewayByKey,
		UDPRoutes: udpRouteByKey,
	}, nil
}

func (a *udpIngressAggregator) addIngress(udpIngress kongv1beta1.UDPIngress) field.ErrorList {
	var ingressClass string
	if _, ok := udpIngress.Annotations[networkingv1beta1.AnnotationIngressClass]; ok {
		ingressClass = udpIngress.Annotations[networkingv1beta1.AnnotationIngressClass]
	} else {
		ingressClass = udpIngress.Name
	}
	for _, rule := range udpIngress.Spec.Rules {
		a.addIngressRule(udpIngress.Namespace, udpIngress.Name, ingressClass, rule, udpIngress.Spec)
	}
	return nil
}

func (a *udpIngressAggregator) addIngressRule(namespace, name, ingressClass string, rule kongv1beta1.UDPIngressRule, iSpec kongv1beta1.UDPIngressSpec) {
	rgKey := ruleGroupKey(fmt.Sprintf("%s/%s", namespace, ingressClass))
	rg, ok := a.ruleGroups[rgKey]
	if !ok {
		rg = &udpIngressRuleGroup{
			namespace:    namespace,
			name:         name,
			ingressClass: ingressClass,
			port:         rule.Port,
		}
		a.ruleGroups[rgKey] = rg
	}
	rg.rules = append(rg.rules, udpIngressRule{rule: rule})
}

func (a *udpIngressAggregator) toRoutesAndGateways() ([]gatewayv1alpha2.UDPRoute, []gatewayv1beta1.Gateway, field.ErrorList) {
	var udpRoutes []gatewayv1alpha2.UDPRoute

	var errors field.ErrorList
	listenersByNamespacedGateway := map[string][]gatewayv1beta1.Listener{}

	for _, rg := range a.ruleGroups {
		listener := gatewayv1beta1.Listener{}
		listener.Port = gatewayv1beta1.PortNumber(rg.port)
		gwKey := fmt.Sprintf("%s/%s", rg.namespace, rg.ingressClass)
		listenersByNamespacedGateway[gwKey] = append(listenersByNamespacedGateway[gwKey], listener)
		var errs field.ErrorList
		var udpRoute gatewayv1alpha2.UDPRoute
		udpRoute, errs = rg.toUDPRoute()
		udpRoutes = append(udpRoutes, udpRoute)

		errors = append(errors, errs...)
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
			gateway.SetGroupVersionKind(common.GatewayGVK)
			gatewaysByKey[gwKey] = gateway
		}
		for _, listener := range listeners {
			gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1beta1.Listener{
				Protocol: gatewayv1beta1.UDPProtocolType,
				Port:     listener.Port,
				Name:     *buildSectionName("udp", strconv.Itoa(int(listener.Port))),
			})
		}
	}

	var gateways []gatewayv1beta1.Gateway
	for _, gw := range gatewaysByKey {
		gateways = append(gateways, *gw)
	}

	return udpRoutes, gateways, errors
}

func (rg *udpIngressRuleGroup) toUDPRoute() (gatewayv1alpha2.UDPRoute, field.ErrorList) {
	var errors field.ErrorList

	udpRoute := gatewayv1alpha2.UDPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RouteName(rg.name, strconv.Itoa(rg.port)),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1alpha2.UDPRouteSpec{},
		Status: gatewayv1alpha2.UDPRouteStatus{
			RouteStatus: gatewayv1beta1.RouteStatus{
				Parents: []gatewayv1beta1.RouteParentStatus{},
			},
		},
	}
	udpRoute.SetGroupVersionKind(common.UDPRouteGVK)

	if rg.ingressClass != "" {
		udpRoute.Spec.ParentRefs = []gatewayv1beta1.ParentReference{
			{
				Name:        gatewayv1beta1.ObjectName(rg.ingressClass),
				SectionName: buildSectionName("udp", strconv.Itoa(rg.port)),
			},
		}
	}

	for _, rule := range rg.rules {
		udpRoute.Spec.Rules = append(udpRoute.Spec.Rules,
			gatewayv1alpha2.UDPRouteRule{
				BackendRefs: []gatewayv1beta1.BackendRef{
					{
						BackendObjectReference: gatewayv1beta1.BackendObjectReference{
							Name: gatewayv1beta1.ObjectName(rule.rule.Backend.ServiceName),
							Port: common.PtrTo(gatewayv1beta1.PortNumber(rule.rule.Backend.ServicePort)),
						},
					},
				},
			},
		)
	}

	return udpRoute, errors
}
