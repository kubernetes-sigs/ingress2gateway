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
	"time"

	networkingv1 "k8s.io/api/networking/v1"
)

// Validates that a service is accessible and working correctly. The addr parameter is a
// "host:port" string representing the service endpoint.
type verifier interface {
	verify(ctx context.Context, log logger, addr string, ingress *networkingv1.Ingress) error
}

type httpGetVerifier struct {
	host string
	path string
}

func (v *httpGetVerifier) verify(ctx context.Context, log logger, addr string, ingress *networkingv1.Ingress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", addr, v.path), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
	}

	// If the Host field is specified in the test case, use that. Otherwise, default to deriving
	// the (auto-generated) host from the ingress.
	if v.host != "" {
		req.Host = v.host
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
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d", res.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading HTTP body: %w", err)
	}

	log.Logf("Got a healthy response: %s", body)

	return nil
}

type httpRedirectVerifier struct {
	host           string
	targetHost     string // What we expect in the Location header
	expectedStatus []int  // Usually 301
}

func (v *httpRedirectVerifier) verify(ctx context.Context, log logger, addr string, ingress *networkingv1.Ingress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/", addr), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
	}

	if v.host != "" {
		req.Host = v.host
	} else if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != "" {
		req.Host = ingress.Spec.Rules[0].Host
	} else {
		return fmt.Errorf("no host specified: set HTTPRedirectVerifier.Host or ensure ingress has a rule with a host")
	}

	client := http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects, we want to inspect the redirect response
			return http.ErrUseLastResponse
		},
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("doing request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	statusValOk := false
	for _, status := range v.expectedStatus {
		if res.StatusCode == status {
			statusValOk = true
			break
		}
	}
	if !statusValOk {

		return fmt.Errorf("unexpected HTTP status code: got %d, want one of %v", res.StatusCode, v.expectedStatus)
	}

	location := res.Header.Get("Location")
	if location == "" {
		return fmt.Errorf("expected Location header, got none")
	}

	expectedLocation := fmt.Sprintf("http://%s", v.targetHost)
	expectedLocationTrailing := fmt.Sprintf("http://%s/", v.targetHost)
	if location != expectedLocation && location != expectedLocationTrailing && location != expectedLocation+":80" && location != expectedLocation+":80/" {
		return fmt.Errorf("unexpected Location header: got %s, want %s (or with trailing slash)", location, expectedLocation)
	}

	log.Logf("Got expected redirect to %s", location)

	return nil
}
