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

package gce

import (
	"context"

	"encoding/json"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/gce/extensions"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
	frontendconfigv1beta1 "k8s.io/ingress-gce/pkg/apis/frontendconfig/v1beta1"
)

type contextKey int

const (
	serviceKey contextKey = iota
)

// resourcesToIRConverter implements the ToIR function of i2gw.ResourcesToIRConverter interface.
type resourcesToIRConverter struct {
	conf *i2gw.ProviderConf

	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
	ctx                           context.Context
}

// newResourcesToIRConverter returns an ingress-gce resourcesToIRConverter instance.
func newResourcesToIRConverter(conf *i2gw.ProviderConf) resourcesToIRConverter {
	return resourcesToIRConverter{
		conf: conf,
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			ToImplementationSpecificHTTPPathTypeMatch: implementationSpecificHTTPPathTypeMatch,
		},
		ctx: context.Background(),
	}
}

func (c *resourcesToIRConverter) convertToIR(storage *storage) (providerir.ProviderIR, field.ErrorList) {
	ingressList := []networkingv1.Ingress{}
	for _, ing := range storage.Ingresses {
		if ing != nil && common.GetIngressClass(*ing) == "" {
			if ing.Annotations == nil {
				ing.Annotations = make(map[string]string)
			}
			ing.Annotations[networkingv1beta1.AnnotationIngressClass] = gceIngressClass
		}
		ingressList = append(ingressList, *ing)
	}

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	ir, errs := common.ToIR(ingressList, storage.ServicePorts, c.implementationSpecificOptions)
	if len(errs) > 0 {
		return providerir.ProviderIR{}, errs
	}

	// Extract gatewayClassName from provider-specific flags
	var gatewayClassName string
	if c.conf != nil && c.conf.ProviderSpecificFlags != nil {
		gatewayClassName = c.conf.ProviderSpecificFlags["gce"][GatewayClassNameFlag]
	}

	if gatewayClassName != "" && !SupportedGKEGatewayClasses[gatewayClassName] {
		errs = append(errs, field.Invalid(field.NewPath("provider-specific-flags"), gatewayClassName, "invalid GCE gateway class name"))
		return providerir.ProviderIR{}, errs
	}

	errs = setGCEGatewayClasses(ingressList, ir.Gateways, gatewayClassName)
	if len(errs) > 0 {
		return providerir.ProviderIR{}, errs
	}
	buildGceGatewayIR(c.ctx, storage, &ir)
	buildGceServiceIR(c.ctx, storage, &ir)
	return ir, errs
}

func buildGceGatewayIR(ctx context.Context, storage *storage, ir *providerir.ProviderIR) {
	if ir.Gateways == nil {
		ir.Gateways = make(map[types.NamespacedName]providerir.GatewayContext)
	}

	feConfigToGwys := getFrontendConfigMapping(ctx, storage)
	for feConfigKey, feConfig := range storage.FrontendConfigs {
		if feConfig == nil {
			continue
		}
		gceGatewayIR := feConfigToGceGatewayIR(feConfig)
		gateways := feConfigToGwys[feConfigKey]

		for _, gwyKey := range gateways {
			gatewayContext := ir.Gateways[gwyKey]
			gatewayContext.ProviderSpecificIR.Gce = &gceGatewayIR
			ir.Gateways[gwyKey] = gatewayContext
		}
	}
}

type gatewayNames []types.NamespacedName

func getFrontendConfigMapping(ctx context.Context, storage *storage) map[types.NamespacedName]gatewayNames {
	feConfigToGwys := make(map[types.NamespacedName]gatewayNames)

	for _, ingress := range storage.Ingresses {
		gwyKey := types.NamespacedName{Namespace: ingress.Namespace, Name: common.GetIngressClass(*ingress)}
		// ing := types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}
		ctx = context.WithValue(ctx, serviceKey, ingress)

		feConfigName, exists := getFrontendConfigAnnotation(ingress)
		if exists {
			feConfigKey := types.NamespacedName{Namespace: ingress.Namespace, Name: feConfigName}
			feConfigToGwys[feConfigKey] = append(feConfigToGwys[feConfigKey], gwyKey)
			continue
		}

	}
	return feConfigToGwys
}

// Get names of the FrontendConfig in the cluster based on the FrontendConfig
// annotation on k8s Services.
func getFrontendConfigAnnotation(ing *networkingv1.Ingress) (string, bool) {
	val, ok := ing.ObjectMeta.Annotations[frontendConfigKey]
	if !ok {
		return "", false
	}
	return val, true
}

func feConfigToGceGatewayIR(feConfig *frontendconfigv1beta1.FrontendConfig) gce.GatewayIR {
	var gceGatewayIR gce.GatewayIR
	if feConfig.Spec.SslPolicy != nil {
		gceGatewayIR.SslPolicy = extensions.BuildIRSslPolicyConfig(feConfig)
	}
	return gceGatewayIR
}

type serviceNames []types.NamespacedName

func buildGceServiceIR(ctx context.Context, storage *storage, ir *providerir.ProviderIR) {
	if ir.Services == nil {
		ir.Services = make(map[types.NamespacedName]providerir.ProviderSpecificServiceIR)
	}

	beConfigToSvcs := getBackendConfigMapping(ctx, storage)
	for beConfigKey, beConfig := range storage.BackendConfigs {
		if beConfig == nil {
			continue
		}
		if err := extensions.ValidateBeConfig(beConfig); err != nil {
			notify(notifications.ErrorNotification, err.Error(), beConfig)
			continue
		}
		gceServiceIR := beConfigToGceServiceIR(beConfig)
		services := beConfigToSvcs[beConfigKey]
		for _, svcKey := range services {
			serviceIR := ir.Services[svcKey]
			serviceIR.Gce = &gceServiceIR
			ir.Services[svcKey] = serviceIR
		}
	}
}

func getBackendConfigMapping(ctx context.Context, storage *storage) map[types.NamespacedName]serviceNames {
	beConfigToSvcs := make(map[types.NamespacedName]serviceNames)

	for _, service := range storage.Services {
		svc := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
		ctx = context.WithValue(ctx, serviceKey, service)

		// Read BackendConfig based on v1 BackendConfigKey.
		beConfigName, exists := getBackendConfigName(ctx, service, backendConfigKey)
		if exists {
			beConfigKey := types.NamespacedName{Namespace: service.Namespace, Name: beConfigName}
			beConfigToSvcs[beConfigKey] = append(beConfigToSvcs[beConfigKey], svc)
			continue
		}

		// Read BackendConfig based on v1beta1 BackendConfigKey.
		beConfigName, exists = getBackendConfigName(ctx, service, betaBackendConfigKey)
		if exists {
			beConfigKey := types.NamespacedName{Namespace: service.Namespace, Name: beConfigName}
			beConfigToSvcs[beConfigKey] = append(beConfigToSvcs[beConfigKey], svc)
			continue
		}
	}
	return beConfigToSvcs
}

// Get names of the BackendConfig in the cluster based on the BackendConfig
// annotation on k8s Services.
func getBackendConfigName(ctx context.Context, service *apiv1.Service, backendConfigKey string) (string, bool) {
	val, exists := getBackendConfigAnnotation(service, backendConfigKey)
	if !exists {
		return "", false
	}

	return parseBackendConfigName(ctx, val)
}

// Get the backend config annotation from the K8s service if it exists.
func getBackendConfigAnnotation(service *apiv1.Service, backendConfigKey string) (string, bool) {
	val, ok := service.Annotations[backendConfigKey]
	if ok {
		return val, ok
	}
	return "", false
}

type backendConfigs struct {
	Default string            `json:"default,omitempty"`
	Ports   map[string]string `json:"ports,omitempty"`
}

// Parse the name of the BackendConfig based on the annotation.
// If different BackendConfigs are used on the same service, pick the one with
// the alphabetically smallest name.
func parseBackendConfigName(ctx context.Context, val string) (string, bool) {
	service := ctx.Value(serviceKey).(*apiv1.Service)

	var configs backendConfigs
	if err := json.Unmarshal([]byte(val), &configs); err != nil {
		notify(notifications.ErrorNotification, "BackendConfig annotation is invalid json", service)
		return "", false
	}

	if configs.Default == "" && len(configs.Ports) == 0 {
		notify(notifications.ErrorNotification, "No BackendConfig's found in annotation", service)
		return "", false
	}

	if len(configs.Ports) != 0 {
		notify(notifications.ErrorNotification, "HealthCheckPolicy and GCPBackendPolicy can only be attached on the whole service, so having a dedicate policy for each port is not yet supported. Picking the first BackendConfig to translate to corresponding Gateway policy.", service)
		// Return the BackendConfig associated with the alphabetically smallest port.
		var backendConfigName string
		var lowestPort string
		for p, name := range configs.Ports {
			if lowestPort == "" || p < lowestPort {
				backendConfigName = name
				lowestPort = p
			}
		}
		return backendConfigName, true
	}
	return configs.Default, true
}

func beConfigToGceServiceIR(beConfig *backendconfigv1.BackendConfig) gce.ServiceIR {
	var gceServiceIR gce.ServiceIR
	if beConfig.Spec.SessionAffinity != nil {
		gceServiceIR.SessionAffinity = extensions.BuildIRSessionAffinityConfig(beConfig)
	}
	if beConfig.Spec.SecurityPolicy != nil {
		gceServiceIR.SecurityPolicy = extensions.BuildIRSecurityPolicyConfig(beConfig)
	}
	if beConfig.Spec.HealthCheck != nil {
		gceServiceIR.HealthCheck = extensions.BuildIRHealthCheckConfig(beConfig)
	}

	return gceServiceIR
}
