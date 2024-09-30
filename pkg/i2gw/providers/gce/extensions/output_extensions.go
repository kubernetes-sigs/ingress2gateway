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

package extensions

import (
	gkegatewayv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
)

func BuildGCPBackendPolicySessionAffinityConfig(serviceIR intermediate.ProviderSpecificServiceIR) *gkegatewayv1.SessionAffinityConfig {
	affinityType := serviceIR.Gce.SessionAffinity.AffinityType
	saConfig := gkegatewayv1.SessionAffinityConfig{
		Type: &affinityType,
	}
	if affinityType == "GENERATED_COOKIE" {
		saConfig.CookieTTLSec = serviceIR.Gce.SessionAffinity.CookieTTLSec
	}
	return &saConfig
}

func BuildGCPBackendPolicySecurityPolicyConfig(serviceIR intermediate.ProviderSpecificServiceIR) *string {
	securityPolicy := serviceIR.Gce.SecurityPolicy.Name
	return &securityPolicy
}

func BuildGCPGatewayPolicySecurityPolicyConfig(gatewayIR intermediate.ProviderSpecificGatewayIR) string {
	return gatewayIR.Gce.SslPolicy.Name
}

func BuildHealthCheckPolicyConfig(serviceIR intermediate.ProviderSpecificServiceIR) *gkegatewayv1.HealthCheckPolicyConfig {
	hcConfig := gkegatewayv1.HealthCheckPolicyConfig{
		CheckIntervalSec:   serviceIR.Gce.HealthCheck.CheckIntervalSec,
		TimeoutSec:         serviceIR.Gce.HealthCheck.TimeoutSec,
		HealthyThreshold:   serviceIR.Gce.HealthCheck.HealthyThreshold,
		UnhealthyThreshold: serviceIR.Gce.HealthCheck.UnhealthyThreshold,
	}
	commonHc := gkegatewayv1.CommonHealthCheck{
		Port: serviceIR.Gce.HealthCheck.Port,
	}
	commonHTTPHc := gkegatewayv1.CommonHTTPHealthCheck{
		RequestPath: serviceIR.Gce.HealthCheck.RequestPath,
	}

	switch *serviceIR.Gce.HealthCheck.Type {
	case "HTTP":
		hcConfig.Config = &gkegatewayv1.HealthCheck{
			Type: gkegatewayv1.HTTP,
			HTTP: &gkegatewayv1.HTTPHealthCheck{
				CommonHealthCheck:     commonHc,
				CommonHTTPHealthCheck: commonHTTPHc,
			},
		}

	case "HTTPS":
		hcConfig.Config = &gkegatewayv1.HealthCheck{
			Type: gkegatewayv1.HTTPS,
			HTTPS: &gkegatewayv1.HTTPSHealthCheck{
				CommonHealthCheck:     commonHc,
				CommonHTTPHealthCheck: commonHTTPHc,
			},
		}

	case "HTTP2":
		hcConfig.Config = &gkegatewayv1.HealthCheck{
			Type: gkegatewayv1.HTTP2,
			HTTP2: &gkegatewayv1.HTTP2HealthCheck{
				CommonHealthCheck:     commonHc,
				CommonHTTPHealthCheck: commonHTTPHc,
			},
		}

	default:
		return nil
	}

	return &hcConfig
}
