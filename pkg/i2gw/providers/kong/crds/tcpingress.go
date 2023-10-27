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
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	configurationv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
)

// TcpIngressToGatewayAPI converts the received TCPingresses to i2gw.GatewayResources,
func TcpIngressToGatewayAPI(ingresses []configurationv1beta1.TCPIngress) (i2gw.GatewayResources, field.ErrorList) {
	aggregator := tcpIngressAggregator{ruleGroups: map[ruleGroupKey]*tcpIngressRuleGroup{}}

	var errs field.ErrorList
	for _, ingress := range ingresses {
		errs = append(errs, aggregator.addIngress(ingress)...)
	}
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	routes, gateways, errs := aggregator.toTCPRoutesAndGateways()
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	routeByKey := make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute)
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
		Gateways:  gatewayByKey,
		TCPRoutes: routeByKey,
	}, nil
}

func (a *tcpIngressAggregator) addIngress(tcpIngress configurationv1beta1.TCPIngress) field.ErrorList {
	var ingressClass string
	if _, ok := tcpIngress.Annotations[networkingv1beta1.AnnotationIngressClass]; ok {
		ingressClass = tcpIngress.Annotations[networkingv1beta1.AnnotationIngressClass]
	} else {
		ingressClass = tcpIngress.Name
	}
	for _, rule := range tcpIngress.Spec.Rules {
		a.addIngressRule(tcpIngress.Namespace, ingressClass, rule, tcpIngress.Spec)
	}
	return nil
}

func (a *tcpIngressAggregator) addIngressRule(namespace, ingressClass string, rule configurationv1beta1.IngressRule, iSpec configurationv1beta1.TCPIngressSpec) {
	rgKey := ruleGroupKey(fmt.Sprintf("%s/%s/%s", namespace, ingressClass, rule.Host))
	rg, ok := a.ruleGroups[rgKey]
	if !ok {
		rg = &tcpIngressRuleGroup{
			namespace:    namespace,
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

func (a *tcpIngressAggregator) toTCPRoutesAndGateways() ([]gatewayv1alpha2.TCPRoute, []gatewayv1beta1.Gateway, field.ErrorList) {
	var tcpRoutes []gatewayv1alpha2.TCPRoute
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
		listener.Port = gatewayv1beta1.PortNumber(rg.port)
		for _, tls := range rg.tls {
			listener.TLS.CertificateRefs = append(listener.TLS.CertificateRefs,
				gatewayv1beta1.SecretObjectReference{Name: gatewayv1beta1.ObjectName(tls.SecretName)})
		}
		gwKey := fmt.Sprintf("%s/%s", rg.namespace, rg.ingressClass)
		listenersByNamespacedGateway[gwKey] = append(listenersByNamespacedGateway[gwKey], listener)
		tcpRoute, errs := rg.toTCPRoute()
		tcpRoutes = append(tcpRoutes, tcpRoute)
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
				Hostname: listener.Hostname,
				Protocol: gatewayv1beta1.TCPProtocolType,
				Port:     listener.Port,
				Name:     *buildSectionName("tcp", string(*listener.Hostname), strconv.Itoa(int(listener.Port))),
			})
			if listener.TLS != nil {
				gateway.Spec.Listeners = append(gateway.Spec.Listeners, gatewayv1beta1.Listener{
					Hostname: listener.Hostname,
					TLS:      listener.TLS,
					Protocol: gatewayv1beta1.TLSProtocolType,
					Name:     "tls",
				})
			}
		}
	}

	var gateways []gatewayv1beta1.Gateway
	for _, gw := range gatewaysByKey {
		gateways = append(gateways, *gw)
	}

	return tcpRoutes, gateways, errors
}

func (rg *tcpIngressRuleGroup) toTCPRoute() (gatewayv1alpha2.TCPRoute, field.ErrorList) {
	var errors field.ErrorList

	tcpRoute := gatewayv1alpha2.TCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NameFromHost(rg.host),
			Namespace: rg.namespace,
		},
		Spec: gatewayv1alpha2.TCPRouteSpec{},
		Status: gatewayv1alpha2.TCPRouteStatus{
			RouteStatus: gatewayv1beta1.RouteStatus{
				Parents: []gatewayv1beta1.RouteParentStatus{},
			},
		},
	}
	tcpRoute.SetGroupVersionKind(common.TCPRouteGVK)

	if rg.ingressClass != "" {
		protocolName := "tcp"
		if rg.tls != nil {
			protocolName = "tls"
		}
		tcpRoute.Spec.ParentRefs = []gatewayv1beta1.ParentReference{
			{
				Name:        gatewayv1beta1.ObjectName(rg.ingressClass),
				SectionName: buildSectionName(protocolName, rg.host, strconv.Itoa(rg.port)),
			},
		}
	}

	for _, rule := range rg.rules {
		tcpRoute.Spec.Rules = append(tcpRoute.Spec.Rules,
			gatewayv1alpha2.TCPRouteRule{
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

	return tcpRoute, errors
}

func buildSectionName(parts ...string) *gatewayv1beta1.SectionName {
	builder := strings.Builder{}
	for i, p := range parts {
		if i != 0 {
			builder.WriteString("-")
		}
		builder.WriteString(p)
	}
	return (*gatewayv1beta1.SectionName)(common.PtrTo(builder.String()))
}
