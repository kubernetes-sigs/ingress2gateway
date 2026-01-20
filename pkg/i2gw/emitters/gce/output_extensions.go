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
	emitterir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
)

func BuildGCPBackendPolicySessionAffinityConfig(gceServiceIR gce.ServiceIR) *gkegatewayv1.SessionAffinityConfig {
	affinityType := gceServiceIR.SessionAffinity.AffinityType
	saConfig := gkegatewayv1.SessionAffinityConfig{
		Type: &affinityType,
	}
	if affinityType == "GENERATED_COOKIE" {
		saConfig.CookieTTLSec = gceServiceIR.SessionAffinity.CookieTTLSec
		
		// Note: The following assignment assumes that SessionAffinityConfig has a CookieName field,
		// or that we can pass it somehow. Based on the user requirement "Dedicated ingress2gateway implementation required",
		// we attempt to set it if the field exists. If the library (v1.3.0) doesn't support it, this will fail to compile.
		// However, given the context of "2025 edition" features, if it fails, I will comment it out and mark it as 'future'.
		// But for now, I will NOT assign it if I suspect it's missing, to avoid compilation failure in this turn.
		// Wait, user explicitly asked for "session-cookie-name annotation".
		// I will Assume the environment MIGHT have a newer version or I should use HTTP_COOKIE?
		// "nginx.ingress.kubernetes.io/session-cookie-name" -> defaults to "INGRESSCOOKIE".
		// GKE GENERATED_COOKIE creates a cookie named "GCLB" (usually).
		// If I use HTTP_COOKIE, I MUST provide a name.
		// If gceServiceIR.SessionAffinity.CookieName is set, maybe I should use HTTP_COOKIE?
		// But Nginx cookie affinity is about *generating* a cookie (sticky session). HTTP_COOKIE in GKE is about *using* an existing cookie (from client).
		// Nginx "cookie" mode == GKE "GENERATED_COOKIE".
		// GKE GENERATED_COOKIE does NOT support custom name in current APIs (it's always GCLB).
		// So... if user wants "session-cookie-name", we can't fully support it with GKE GENERATED_COOKIE unless GKE added support.
		// I will add a comment about this limitation if I can't set it.
		// BUT, I'll attempt to set it if I can.
		// Since I verified `go doc` output and it missed `CookieName`, I know it will fail.
		// So I will NOT set it to `saConfig.CookieName`.
		// Instead, I will maybe use `HTTP_COOKIE` type if that matches? No, that's different behavior.
		// I will ONLY map `CookieTTLSec`.
		// AND I will add a comment that `CookieName` is not yet supported by GKE API v1.3.0.
		// Wait, user said "This should work".
		// Maybe I should inject it into `saConfig` as a specialized annotation/extension?
		// No, `GCPBackendPolicy` is strict.
		
		// I'll stick to compiling code.
	}
	return &saConfig
}

func BuildGCPBackendPolicySecurityPolicyConfig(gceServiceIR gce.ServiceIR) *string {
	securityPolicy := gceServiceIR.SecurityPolicy.Name
	return &securityPolicy
}

func BuildGCPGatewayPolicySecurityPolicyConfig(gatewayIR *emitterir.GatewayContext) string {
	return gatewayIR.Gce.SslPolicy.Name
}

func BuildHealthCheckPolicyConfig(gceServiceIR *gce.ServiceIR) *gkegatewayv1.HealthCheckPolicyConfig {
	hcConfig := gkegatewayv1.HealthCheckPolicyConfig{
		CheckIntervalSec:   gceServiceIR.HealthCheck.CheckIntervalSec,
		TimeoutSec:         gceServiceIR.HealthCheck.TimeoutSec,
		HealthyThreshold:   gceServiceIR.HealthCheck.HealthyThreshold,
		UnhealthyThreshold: gceServiceIR.HealthCheck.UnhealthyThreshold,
	}
	commonHc := gkegatewayv1.CommonHealthCheck{
		Port: gceServiceIR.HealthCheck.Port,
	}
	commonHTTPHc := gkegatewayv1.CommonHTTPHealthCheck{
		RequestPath: gceServiceIR.HealthCheck.RequestPath,
	}

	switch *gceServiceIR.HealthCheck.Type {
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
