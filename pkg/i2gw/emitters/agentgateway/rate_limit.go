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

package agentgateway

import (
	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
)

// applyRateLimitPolicy projects the rate limit policy IR into an AgentgatewayPolicy,
// returning true if it modified/created an AgentgatewayPolicy for this ingress.
//
// Agentgateway uses LocalRateLimit with Requests/Tokens and Unit (Seconds/Minutes/Hours),
// which is simpler than kgateway's TokenBucket approach.
func applyRateLimitPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.RateLimit == nil {
		return false
	}

	rl := pol.RateLimit
	if rl.Limit <= 0 {
		return false
	}

	// Default burst multiplier to 1 if unset/zero.
	burstMult := rl.BurstMultiplier
	if burstMult <= 0 {
		burstMult = 1
	}

	var (
		requests *int32
		unit     agentgatewayv1alpha1.LocalRateLimitUnit
	)

	switch rl.Unit {
	case emitterir.RateLimitUnitRPS:
		// Requests per second.
		requests = &rl.Limit
		unit = agentgatewayv1alpha1.LocalRateLimitUnitSeconds
	case emitterir.RateLimitUnitRPM:
		// Requests per minute.
		requests = &rl.Limit
		unit = agentgatewayv1alpha1.LocalRateLimitUnitMinutes
	default:
		// Unknown unit; ignore for now.
		return false
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)

	// Initialize Traffic section if needed
	if agp.Spec.Traffic == nil {
		agp.Spec.Traffic = &agentgatewayv1alpha1.Traffic{}
	}

	// Initialize RateLimit if needed
	if agp.Spec.Traffic.RateLimit == nil {
		agp.Spec.Traffic.RateLimit = &agentgatewayv1alpha1.RateLimits{}
	}

	// Initialize Local rate limits slice if needed
	if agp.Spec.Traffic.RateLimit.Local == nil {
		agp.Spec.Traffic.RateLimit.Local = []agentgatewayv1alpha1.LocalRateLimit{}
	}

	// Create LocalRateLimit entry
	localRateLimit := agentgatewayv1alpha1.LocalRateLimit{
		Requests: requests,
		Unit:     unit,
	}

	// Set burst if multiplier is greater than 1
	if burstMult > 1 {
		burst := rl.Limit * burstMult
		localRateLimit.Burst = &burst
	}

	// Append to Local rate limits
	agp.Spec.Traffic.RateLimit.Local = append(agp.Spec.Traffic.RateLimit.Local, localRateLimit)

	return true
}
