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

type Verifier interface {
	Verify(ctx context.Context, log logger, addr Addresses, ingress *networkingv1.Ingress) error
}

type Addresses struct {
	HTTP  string
	HTTPS string
}

type CanaryVerifier struct {
	Verifier Verifier
	MinSuccesses float64
	MaxSuccesses float64
	Runs int
}

func (v *CanaryVerifier) Verify(ctx context.Context, log logger, addr Addresses, ingress *networkingv1.Ingress) error {
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
	Code int
	BodyPrefix string // Check that the body starts with this prefix
	BodyIncludes []string
	HeaderMatches []HeaderMatch
	UseTLS bool
	CACertPEM []byte
}

type HeaderMatch struct {
	Name string
	Pattern string
}

func (v *HttpGetVerifier) Verify(ctx context.Context, log logger, addr Addresses, ingress *networkingv1.Ingress) error {
	host := v.Host
	if host == "" && len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != "" {
		host = ingress.Spec.Rules[0].Host
	}
	if host == "" {
		return fmt.Errorf("no host specified: set HTTPGetVerifier.Host or ensure ingress has a rule with a host")
	}

	scheme := "http"
	targetAddr := addr.HTTP
	if v.UseTLS {
		scheme = "https"
		targetAddr = addr.HTTPS
	}
	if targetAddr == "" {
		return fmt.Errorf("no %s address available for verifier", scheme)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s://%s%s", scheme, targetAddr, v.Path), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
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

	if v.Code == 0 {
		v.Code = http.StatusOK
	}
	if res.StatusCode != v.Code {
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d", res.StatusCode, v.Code)
	}

	for _, headerMatch := range v.HeaderMatches {
		if headerMatch.Name == "" {
			return fmt.Errorf("header match name cannot be empty")
		}
		pattern, err := regexp.Compile(headerMatch.Pattern)
		if err != nil {
			return fmt.Errorf("invalid header regex for %s: %w", headerMatch.Name, err)
		}
		values := res.Header.Values(headerMatch.Name)
		if len(values) == 0 {
			return fmt.Errorf("missing header %q on response", headerMatch.Name)
		}
		matched := false
		for _, value := range values {
			if pattern.MatchString(value) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("header %q did not match %q (got %q)", headerMatch.Name, headerMatch.Pattern, strings.Join(values, ", "))
		}
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
