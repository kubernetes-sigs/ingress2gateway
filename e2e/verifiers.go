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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
)

type verifier interface {
	verify(ctx context.Context, log logger, addr addresses, ingress *networkingv1.Ingress) error
}

type addresses struct {
	http  string
	https string
}

type canaryVerifier struct {
	verifier     verifier
	minSuccesses float64
	maxSuccesses float64
	runs         int
}

func (v *canaryVerifier) verify(ctx context.Context, log logger, addr addresses, ingress *networkingv1.Ingress) error {
	successes := 0
	for i := 0; i < v.runs; i++ {
		err := v.verifier.verify(ctx, log, addr, ingress)
		if err != nil {
			log.Logf("Canary verifier run %d/%d succeeded", i+1, v.runs)
			successes++
		}
	}

	successRate := float64(successes) / float64(v.runs)
	if successRate <= v.minSuccesses || successRate >= v.maxSuccesses {
		return fmt.Errorf("canary verifier failed: success rate %.2f not in range [%.2f, %.2f]", successRate, v.minSuccesses, v.maxSuccesses)
	}
	return nil
}

type httpGetVerifier struct {
	host           string
	path           string
	method         string
	requestHeaders map[string]string
	allowedCodes   []int
	headerMatches  []headerMatch
	headerExcludes []headerExclude
	headerAbsent   []string
	useTLS         bool
	caCertPEM      []byte
	bodyRegex      *regexp.Regexp
}

type headerMatch struct {
	name     string
	patterns []*regexp.Regexp
}

type headerExclude struct {
	name     string
	patterns []*regexp.Regexp
}

func (v *httpGetVerifier) verify(ctx context.Context, log logger, addr addresses, ingress *networkingv1.Ingress) error {
	host := v.host
	if host == "" && len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != "" {
		host = ingress.Spec.Rules[0].Host
	}
	if host == "" {
		return fmt.Errorf("no host specified: set HTTPGetVerifier.Host or ensure ingress has a rule with a host")
	}

	scheme := "http"
	targetAddr := addr.http
	if v.useTLS {
		scheme = "https"
		targetAddr = addr.https
	}
	if targetAddr == "" {
		return fmt.Errorf("no %s address available for verifier", scheme)
	}
	method := v.method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s://%s%s", scheme, targetAddr, v.path), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
	}

	for name, value := range v.requestHeaders {
		req.Header.Set(name, value)
	}
	req.Host = host

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if v.useTLS {
		if len(v.caCertPEM) == 0 {
			return fmt.Errorf("no CA cert provided for TLS verification")
		}
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM(v.caCertPEM); !ok {
			return fmt.Errorf("failed to parse CA cert PEM")
		}
		transport.TLSClientConfig = &tls.Config{
			RootCAs:    certPool,
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
	}

	client := http.Client{Timeout: 5 * time.Second, Transport: transport}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("doing request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	allowedCodes := v.allowedCodes
	if len(allowedCodes) == 0 {
		allowedCodes = []int{http.StatusOK}
	}
	allowed := false
	for _, code := range allowedCodes {
		if res.StatusCode == code {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("unexpected HTTP status code: got %d, want one of %v", res.StatusCode, allowedCodes)
	}

	for _, headerMatch := range v.headerMatches {
		if headerMatch.name == "" {
			return fmt.Errorf("header match name cannot be empty")
		}
		if len(headerMatch.patterns) == 0 {
			return fmt.Errorf("header match patterns cannot be empty for %q", headerMatch.name)
		}
		values := res.Header.Values(headerMatch.name)
		if len(values) == 0 {
			return fmt.Errorf("missing header %q on response", headerMatch.name)
		}
		for _, pattern := range headerMatch.patterns {
			matched := false
			for _, value := range values {
				if pattern.MatchString(value) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("header %q did not match %q (got %q)", headerMatch.name, pattern, strings.Join(values, ", "))
			}
		}
	}

	for _, headerExclude := range v.headerExcludes {
		if headerExclude.name == "" {
			return fmt.Errorf("header exclude name cannot be empty")
		}
		if len(headerExclude.patterns) == 0 {
			return fmt.Errorf("header exclude patterns cannot be empty for %q", headerExclude.name)
		}
		values := res.Header.Values(headerExclude.name)
		if len(values) == 0 {
			continue
		}
		for _, pattern := range headerExclude.patterns {
			for _, value := range values {
				if pattern.MatchString(value) {
					return fmt.Errorf("header %q matched excluded pattern %q (got %q)", headerExclude.name, pattern, strings.Join(values, ", "))
				}
			}
		}
	}

	for _, headerName := range v.headerAbsent {
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
	log.Logf("Got a healthy response: %s", body)

	if v.bodyRegex != nil && !v.bodyRegex.MatchString(string(body)) {
		return fmt.Errorf("unexpected HTTP body: does not match %v", v.bodyRegex)
	}

	return nil
}
