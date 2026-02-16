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

package framework

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Verifier validates that a service is accessible and working correctly. The addr parameter is a
// "host:port" string representing the service endpoint. The defaultHost parameter is the host to
// use if the verifier does not have a host configured.
type Verifier interface {
	verify(ctx context.Context, log Logger, addr addresses, defaultHost string) error
}

// Holds HTTP and HTTPS addresses for a service.
type addresses struct {
	http  string
	https string
}

// CanaryVerifier runs a verifier multiple times and checks the success rate.
type CanaryVerifier struct {
	Verifier     Verifier
	MinSuccesses float64
	MaxSuccesses float64
	Runs         int
}

func (v *CanaryVerifier) verify(ctx context.Context, log Logger, addr addresses, defaultHost string) error {
	successes := 0
	for i := 0; i < v.Runs; i++ {
		err := v.Verifier.verify(ctx, log, addr, defaultHost)
		if err == nil {
			log.Logf("Canary verifier run %d/%d for host %q succeeded", i+1, v.Runs, defaultHost)
			successes++
		}
	}

	successRate := float64(successes) / float64(v.Runs)
	if successRate <= v.MinSuccesses || successRate >= v.MaxSuccesses {
		return fmt.Errorf("canary verifier failed: success rate %.2f not in range [%.2f, %.2f]", successRate, v.MinSuccesses, v.MaxSuccesses)
	}
	return nil
}

// HTTPRequestVerifier makes an HTTP or HTTPS request to the ingress and validates the response
// based on the provided configuration. The fields that check the response are optional but are
// ANDed together if set.
type HTTPRequestVerifier struct {
	// Host is the Host header/SNI to use in the request. If empty, the verifier will attempt to
	// infer it from the ingress rules.
	Host string
	// Path is the URL path to request (default "/")
	Path string
	// Method is the HTTP method to use (default GET)
	Method string
	// RequestHeaders are additional headers to include in the request
	RequestHeaders map[string]string
	// AllowedCodes are the expected HTTP status codes (default 200)
	AllowedCodes []int
	// HeaderMatches specifies headers that must be present and match at all of the provided regex
	// patterns
	HeaderMatches []HeaderMatch
	// HeaderAbsent specifies headers that must not be present in the response
	HeaderAbsent []string
	// UseTLS indicates whether to use HTTPS instead of HTTP
	UseTLS bool
	// CACertPEM is the PEM-encoded CA certificate to trust for TLS verification (required if
	// UseTLS is true)
	CACertPEM []byte
	// BodyRegex is an optional regex pattern that the response body must match
	BodyRegex *regexp.Regexp
}

// MaybeNegativePattern represents a regex pattern that can be negated. If Negate is true, the
// pattern must NOT match.
type MaybeNegativePattern struct {
	Pattern *regexp.Regexp
	Negate  bool
}

func (m MaybeNegativePattern) matches(s string) bool {
	return m.Pattern.MatchString(s) != m.Negate
}

// HeaderMatch specifies a header name and patterns that must match.
type HeaderMatch struct {
	Name     string
	Patterns []*MaybeNegativePattern
}

func (v *HTTPRequestVerifier) verify(ctx context.Context, log Logger, addr addresses, defaultHost string) error {
	host := v.Host
	if host == "" {
		host = defaultHost
	}
	if host == "" {
		return fmt.Errorf("no host specified: set httpRequestVerifier.host or provide a defaultHost")
	}

	scheme := "http"
	targetAddr := addr.http
	if v.UseTLS {
		scheme = "https"
		targetAddr = addr.https
	}
	if targetAddr == "" {
		return fmt.Errorf("no %s address available for verifier", scheme)
	}
	method := v.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s://%s%s", scheme, targetAddr, v.Path), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
	}

	for name, value := range v.RequestHeaders {
		req.Header.Set(name, value)
	}
	req.Host = host

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if v.UseTLS {
		if len(v.CACertPEM) == 0 {
			return fmt.Errorf("no CA cert provided for TLS verification")
		}
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM(v.CACertPEM); !ok {
			return fmt.Errorf("failed to parse CA cert PEM")
		}
		transport.TLSClientConfig = &tls.Config{
			RootCAs:    certPool,
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
	}

	client := http.Client{Timeout: 20 * time.Second, Transport: transport}
	// Don't follow redirects, as some tests want to verify the redirect response itself (e.g. for TLS redirection)
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("doing request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	allowedCodes := v.AllowedCodes
	if len(allowedCodes) == 0 {
		allowedCodes = []int{http.StatusOK}
	}
	if !slices.Contains(allowedCodes, res.StatusCode) {
		return fmt.Errorf("unexpected HTTP status code: got %d, want one of %v", res.StatusCode, allowedCodes)
	}

	for _, headerMatch := range v.HeaderMatches {
		if headerMatch.Name == "" {
			return fmt.Errorf("header match name cannot be empty")
		}
		if len(headerMatch.Patterns) == 0 {
			return fmt.Errorf("header match patterns cannot be empty for %q", headerMatch.Name)
		}
		values := res.Header.Values(headerMatch.Name)
		if len(values) == 0 {
			return fmt.Errorf("missing header %q on response", headerMatch.Name)
		}
		for _, pattern := range headerMatch.Patterns {
			matched := false
			for _, value := range values {
				if pattern.matches(value) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf(
					"header %q did not match pattern %q with negation %v: header values were %q",
					headerMatch.Name,
					pattern.Pattern,
					pattern.Negate,
					strings.Join(values, ", "),
				)
			}
		}
	}

	for _, headerName := range v.HeaderAbsent {
		if headerName == "" {
			return fmt.Errorf("header absent name cannot be empty")
		}
		if len(res.Header.Values(headerName)) > 0 {
			return fmt.Errorf("unexpected header %q on response", headerName)
		}
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading HTTP body: %w", err)
	}

	log.Logf("Got a healthy response for host %q: %s", host, body)

	if v.BodyRegex != nil && !v.BodyRegex.MatchString(string(body)) {
		return fmt.Errorf("unexpected HTTP body: does not match %v", v.BodyRegex)
	}

	return nil
}
