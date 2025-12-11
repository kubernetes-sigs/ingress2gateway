/*
Copyright 2025 The Kubernetes Authors.

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
	gkegatewayv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
)

func BuildGCPBackendPolicySessionAffinityConfig(serviceIR gce.ServiceIR) *gkegatewayv1.SessionAffinityConfig {
	affinityType := serviceIR.SessionAffinity.AffinityType
	saConfig := gkegatewayv1.SessionAffinityConfig{
		Type: &affinityType,
	}
	if affinityType == "GENERATED_COOKIE" {
		saConfig.CookieTTLSec = serviceIR.SessionAffinity.CookieTTLSec
	}
	return &saConfig
}

func BuildGCPBackendPolicySecurityPolicyConfig(serviceIR gce.ServiceIR) *string {
	securityPolicy := serviceIR.SecurityPolicy.Name
	return &securityPolicy
}

func BuildGCPGatewayPolicySecurityPolicyConfig(gatewayIR *emitterir.GatewayContext) string {
	return gatewayIR.Gce.SslPolicy.Name
}

func BuildHealthCheckPolicyConfig(serviceIR *gce.ServiceIR) *gkegatewayv1.HealthCheckPolicyConfig {
	hcConfig := gkegatewayv1.HealthCheckPolicyConfig{
		CheckIntervalSec:   serviceIR.HealthCheck.CheckIntervalSec,
		TimeoutSec:         serviceIR.HealthCheck.TimeoutSec,
		HealthyThreshold:   serviceIR.HealthCheck.HealthyThreshold,
		UnhealthyThreshold: serviceIR.HealthCheck.UnhealthyThreshold,
	}
	commonHc := gkegatewayv1.CommonHealthCheck{
		Port: serviceIR.HealthCheck.Port,
	}
	commonHTTPHc := gkegatewayv1.CommonHTTPHealthCheck{
		RequestPath: serviceIR.HealthCheck.RequestPath,
	}

	switch *serviceIR.HealthCheck.Type {
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
