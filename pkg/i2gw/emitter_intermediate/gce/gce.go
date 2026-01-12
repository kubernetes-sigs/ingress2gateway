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

type GatewayIR struct {
	EnableHTTPSRedirect bool
	SslPolicy           *SslPolicyConfig
}
type SslPolicyConfig struct {
	Name string
}
type HTTPRouteIR struct{}
type ServiceIR struct {
	SessionAffinity *SessionAffinityConfig
	SecurityPolicy  *SecurityPolicyConfig
	HealthCheck     *HealthCheckConfig
}
type SessionAffinityConfig struct {
	AffinityType string
	CookieTTLSec *int64
}
type SecurityPolicyConfig struct {
	Name string
}
type HealthCheckConfig struct {
	CheckIntervalSec   *int64
	TimeoutSec         *int64
	HealthyThreshold   *int64
	UnhealthyThreshold *int64
	Type               *string
	Port               *int64
	RequestPath        *string
}
