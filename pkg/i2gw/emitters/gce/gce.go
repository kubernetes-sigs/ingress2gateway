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

package gce_emitter

import (
	gkegatewayv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/utils"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providergce "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/gce"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const emitterName = "gce"

var (
	GCPBackendPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "GCPBackendPolicy",
	}

	GCPGatewayPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "GCPGatewayPolicy",
	}

	HealthCheckPolicyGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "HealthCheckPolicy",
	}

	GCPHTTPFilterGVK = schema.GroupVersionKind{
		Group:   "networking.gke.io",
		Version: "v1",
		Kind:    "GCPHTTPFilter",
	}
)

func init() {
	i2gw.EmitterConstructorByName[emitterName] = NewEmitter
}

type Emitter struct {
	notify notifications.NotifyFunc
}

func NewEmitter(conf *i2gw.EmitterConf) i2gw.Emitter {
	return &Emitter{
		notify: conf.Report.Notifier(emitterName),
	}
}

func (c *Emitter) Emit(ir emitterir.EmitterIR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := utils.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	buildGceGatewayExtensions(c.notify, ir, &gatewayResources)
	buildGceServiceExtensions(c.notify, ir, &gatewayResources)
	patchHTTPRoutesWithFilters(ir, &gatewayResources)

	return gatewayResources, nil
}

func buildGceGatewayExtensions(notify notifications.NotifyFunc, ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) {
	for gwyKey, gatewayContext := range ir.Gateways {
		gwyPolicy := addGatewayPolicyIfConfigured(gwyKey, &gatewayContext)
		if gwyPolicy == nil {
			continue
		}
		obj, err := i2gw.CastToUnstructured(gwyPolicy)
		if err != nil {
			notify(notifications.ErrorNotification, "Failed to cast GCPGatewayPolicy to unstructured", gwyPolicy)
			continue
		}
		gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
	}
}

func addGatewayPolicyIfConfigured(gatewayNamespacedName types.NamespacedName, gatewayIR *emitterir.GatewayContext) *gkegatewayv1.GCPGatewayPolicy {
	if gatewayIR == nil || gatewayIR.Gce == nil {
		return nil
	}
	// If there is no specification related to GCPGatewayPolicy feature, return nil.
	if gatewayIR.Gce.SslPolicy == nil {
		return nil
	}
	gcpGatewayPolicy := gkegatewayv1.GCPGatewayPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: gatewayNamespacedName.Namespace,
			Name:      gatewayNamespacedName.Name,
		},
		Spec: gkegatewayv1.GCPGatewayPolicySpec{
			Default: &gkegatewayv1.GCPGatewayPolicyConfig{},
			TargetRef: gatewayv1alpha2.NamespacedPolicyTargetReference{
				Group: "gateway.networking.k8s.io",
				Kind:  "Gateway",
				Name:  gatewayv1.ObjectName(gatewayNamespacedName.Name),
			},
		},
	}
	gcpGatewayPolicy.SetGroupVersionKind(GCPGatewayPolicyGVK)
	if gatewayIR.Gce.SslPolicy != nil {
		gcpGatewayPolicy.Spec.Default.SslPolicy = BuildGCPGatewayPolicySecurityPolicyConfig(gatewayIR)
	}
	return &gcpGatewayPolicy
}

func buildGceServiceExtensions(notify notifications.NotifyFunc, ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) {
	svcKeys := make(map[types.NamespacedName]bool)
	for k := range ir.GceServices {
		svcKeys[k] = true
	}
	for k := range ir.Services {
		svcKeys[k] = true
	}

	for svcKey := range svcKeys {
		var gceServiceIR *gce.ServiceIR
		if gceIR, ok := ir.GceServices[svcKey]; ok {
			gceServiceIR = &gceIR
		}
		var genericServiceIR *emitterir.ServiceContext
		if genIR, ok := ir.Services[svcKey]; ok {
			genericServiceIR = &genIR
		}

		bePolicy := addGCPBackendPolicyIfConfigured(svcKey, genericServiceIR, gceServiceIR)
		if bePolicy != nil {
			obj, err := i2gw.CastToUnstructured(bePolicy)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast GCPBackendPolicy to unstructured", bePolicy)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}

		hcPolicy := addHealthCheckPolicyIfConfigured(svcKey, gceServiceIR)
		if hcPolicy != nil {
			obj, err := i2gw.CastToUnstructured(hcPolicy)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast HealthCheckPolicy to unstructured", hcPolicy)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}

		httpFilter := addGCPHTTPFilterIfConfigured(svcKey, gceServiceIR)
		if httpFilter != nil {
			obj, err := i2gw.CastToUnstructured(httpFilter)
			if err != nil {
				notify(notifications.ErrorNotification, "Failed to cast GCPHTTPFilter to unstructured", httpFilter)
				continue
			}
			gatewayResources.GatewayExtensions = append(gatewayResources.GatewayExtensions, *obj)
		}
	}
}

func addGCPBackendPolicyIfConfigured(serviceNamespacedName types.NamespacedName, genericServiceIR *emitterir.ServiceContext, gceServiceIR *gce.ServiceIR) *gkegatewayv1.GCPBackendPolicy {
	// If there is no specification related to GCPBackendPolicy feature, return nil.
	var hasSessionAffinity bool
	if genericServiceIR != nil && genericServiceIR.SessionAffinity != nil {
		hasSessionAffinity = true
	} else if gceServiceIR != nil && gceServiceIR.SessionAffinity != nil {
		hasSessionAffinity = true
	}

	var hasSecurityPolicy bool
	if gceServiceIR != nil && gceServiceIR.SecurityPolicy != nil {
		hasSecurityPolicy = true
	}

	if !hasSessionAffinity && !hasSecurityPolicy {
		return nil
	}

	gcpBackendPolicy := gkegatewayv1.GCPBackendPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceNamespacedName.Namespace,
			Name:      serviceNamespacedName.Name,
		},
		Spec: gkegatewayv1.GCPBackendPolicySpec{
			Default: &gkegatewayv1.GCPBackendPolicyConfig{},
			TargetRef: gatewayv1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  "Service",
				Name:  gatewayv1.ObjectName(serviceNamespacedName.Name),
			},
		},
	}
	gcpBackendPolicy.SetGroupVersionKind(GCPBackendPolicyGVK)

	if hasSessionAffinity {
		gcpBackendPolicy.Spec.Default.SessionAffinity = BuildGCPBackendPolicySessionAffinityConfig(genericServiceIR, gceServiceIR)
	}
	if hasSecurityPolicy {
		gcpBackendPolicy.Spec.Default.SecurityPolicy = BuildGCPBackendPolicySecurityPolicyConfig(*gceServiceIR)
	}

	return &gcpBackendPolicy
}

func addHealthCheckPolicyIfConfigured(serviceNamespacedName types.NamespacedName, gceServiceIR *gce.ServiceIR) *gkegatewayv1.HealthCheckPolicy {
	if gceServiceIR == nil {
		return nil
	}
	// If there is no specification related to HealthCheckPolicy feature, return nil.
	if gceServiceIR.HealthCheck == nil {
		return nil
	}

	healthCheckPolicy := gkegatewayv1.HealthCheckPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceNamespacedName.Namespace,
			Name:      serviceNamespacedName.Name,
		},
		Spec: gkegatewayv1.HealthCheckPolicySpec{
			Default: BuildHealthCheckPolicyConfig(gceServiceIR),
			TargetRef: gatewayv1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  "Service",
				Name:  gatewayv1.ObjectName(serviceNamespacedName.Name),
			},
		},
	}
	healthCheckPolicy.SetGroupVersionKind(HealthCheckPolicyGVK)
	return &healthCheckPolicy
}

func addGCPHTTPFilterIfConfigured(serviceNamespacedName types.NamespacedName, gceServiceIR *gce.ServiceIR) *providergce.GCPHTTPFilter {
	if gceServiceIR == nil || gceServiceIR.Cdn == nil {
		return nil
	}

	httpFilter := providergce.GCPHTTPFilter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceNamespacedName.Namespace,
			Name:      serviceNamespacedName.Name + "-filter",
		},
		Spec: providergce.GCPHTTPFilterSpec{
			CachePolicy: gceServiceIR.Cdn.CachePolicy,
		},
	}
	httpFilter.SetGroupVersionKind(GCPHTTPFilterGVK)
	return &httpFilter
}

func patchHTTPRoutesWithFilters(ir emitterir.EmitterIR, gatewayResources *i2gw.GatewayResources) {
	for routeKey, route := range gatewayResources.HTTPRoutes {
		for i := range route.Spec.Rules {
			for j := range route.Spec.Rules[i].BackendRefs {
				backendRef := route.Spec.Rules[i].BackendRefs[j]
				if backendRef.Name == "" {
					continue
				}
				svcKey := types.NamespacedName{Namespace: routeKey.Namespace, Name: string(backendRef.Name)}
				gceSvc, exists := ir.GceServices[svcKey]
				if !exists || gceSvc.Cdn == nil {
					continue
				}
				// Found a backend with CDN enabled!
				// Attach the filter to the HTTPRoute rule.
				filter := gatewayv1.HTTPRouteFilter{
					Type: gatewayv1.HTTPRouteFilterExtensionRef,
					ExtensionRef: &gatewayv1.LocalObjectReference{
						Group: "networking.gke.io",
						Kind:  "GCPHTTPFilter",
						Name:  gatewayv1.ObjectName(string(backendRef.Name) + "-filter"),
					},
				}
				if route.Spec.Rules[i].Filters == nil {
					route.Spec.Rules[i].Filters = make([]gatewayv1.HTTPRouteFilter, 0)
				}
				// Avoid adding duplicate filters
				alreadyExists := false
				for _, existingFilter := range route.Spec.Rules[i].Filters {
					if existingFilter.Type == gatewayv1.HTTPRouteFilterExtensionRef &&
						existingFilter.ExtensionRef != nil &&
						existingFilter.ExtensionRef.Name == filter.ExtensionRef.Name {
						alreadyExists = true
						break
					}
				}
				if !alreadyExists {
					route.Spec.Rules[i].Filters = append(route.Spec.Rules[i].Filters, filter)
				}
			}
		}
		gatewayResources.HTTPRoutes[routeKey] = route
	}
}
