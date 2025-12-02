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

package intermediate

// GceGatewayIR contains GCE-specific fields for Gateway.
type GceGatewayIR struct {
	EnableHTTPSRedirect bool
	SslPolicy           *SslPolicyConfig
}

// SslPolicyConfig holds the SSL policy configuration for GCE Gateway.
type SslPolicyConfig struct {
	Name string
}

// GceHTTPRouteIR contains GCE-specific fields for HTTPRoute.
type GceHTTPRouteIR struct{}

// GceServiceIR contains GCE-specific fields for Service.
type GceServiceIR struct {
	SessionAffinity *SessionAffinityConfig
	SecurityPolicy  *SecurityPolicyConfig
	HealthCheck     *HealthCheckConfig
}

// SessionAffinityConfig holds the session affinity configuration for GCE Service.
type SessionAffinityConfig struct {
	AffinityType string
	CookieTTLSec *int64
}

// SecurityPolicyConfig holds the security policy configuration for GCE Service.
type SecurityPolicyConfig struct {
	Name string
}

// HealthCheckConfig holds the health check configuration for GCE Service.
type HealthCheckConfig struct {
	CheckIntervalSec   *int64
	TimeoutSec         *int64
	HealthyThreshold   *int64
	UnhealthyThreshold *int64
	Type               *string
	Port               *int64
	RequestPath        *string
}

func mergeGceGatewayIR(current, existing *GceGatewayIR) *GceGatewayIR {
	// If either GceGatewayIR is nil, return the other one as the merged result.
	if current == nil {
		return existing
	}
	if existing == nil {
		return current
	}

	// If both GceGatewayIRs are not nil, merge their fields.
	var mergedGatewayIR GceGatewayIR
	mergedGatewayIR.EnableHTTPSRedirect = current.EnableHTTPSRedirect || existing.EnableHTTPSRedirect
	return &mergedGatewayIR
}
