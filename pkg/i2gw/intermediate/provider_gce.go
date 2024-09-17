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

type GceGatewayIR struct {
	EnableHTTPSRedirect bool
	SslPolicy           *SslPolicyConfig
}
type SslPolicyConfig struct {
	Name string
}
type GceHTTPRouteIR struct{}
type GceServiceIR struct {
	SessionAffinity *SessionAffinityConfig
	SecurityPolicy  *SecurityPolicyConfig
}
type SessionAffinityConfig struct {
	AffinityType string
	CookieTTLSec *int64
}
type SecurityPolicyConfig struct {
	Name string
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
