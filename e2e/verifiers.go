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

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
)

// Validates that a service is accessible and working correctly. The addr parameter is a
// "host:port" string representing the service endpoint.
type Verifier interface {
	Verify(ctx context.Context, log logger, addr string, ingress *networkingv1.Ingress) error
}

type CanaryVerifier struct {
	Verifier Verifier
	MinSuccesses float64
	MaxSuccesses float64
	Runs int
}

func (v *CanaryVerifier) Verify(ctx context.Context, log logger, addr string, ingress *networkingv1.Ingress) error {
	successes := 0
	for i := 0; i < v.Runs; i++ {
		err := v.Verifier.Verify(ctx, log, addr, ingress)
		if err != nil {
			log.Logf("Canary verifier run %d/%d succeeded", i+1, v.Runs)
			successes++
		}
	}
	
	successRate := float64(successes) / float64(v.Runs)
	if successRate <= v.MinSuccesses || successRate >= v.MaxSuccesses {
		return fmt.Errorf("canary verifier failed: success rate %.2f not in range [%.2f, %.2f]", successRate, v.MinSuccesses, v.MaxSuccesses)
	}
	return nil
}

type HttpGetVerifier struct {
	Host string
	Path string
	BodyPrefix string // Check that the body starts with this prefix
	BodyIncludes []string
}

func (v *HttpGetVerifier) Verify(ctx context.Context, log logger, addr string, ingress *networkingv1.Ingress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", addr, v.Path), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
	}

	// If the Host field is specified in the test case, use that. Otherwise, default to deriving
	// the (auto-generated) host from the ingress.
	if v.Host != "" {
		req.Host = v.Host
	} else if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != "" {
		req.Host = ingress.Spec.Rules[0].Host
	} else {
		return fmt.Errorf("no host specified: set HTTPGetVerifier.Host or ensure ingress has a rule with a host")
	}

	client := http.Client{Timeout: 5 * time.Second}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("doing request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d", res.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading HTTP body: %w", err)
	}
	log.Logf("Got a healthy response: %s", body)

	if !strings.HasPrefix(string(body), v.BodyPrefix) {
		return fmt.Errorf("unexpected HTTP body: does not start with %q", v.BodyPrefix)
	}
	
	for _, include := range v.BodyIncludes {
		if !strings.Contains(string(body), include) {
			return fmt.Errorf("unexpected HTTP body: does not include %q", include)
		}
	}

	return nil
}
