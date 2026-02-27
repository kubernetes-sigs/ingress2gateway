/*
Copyright 2023 The Kubernetes Authors.

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

package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	pb "sigs.k8s.io/gateway-api/conformance/echo-basic/grpcechoserver"
	gwtests "sigs.k8s.io/gateway-api/conformance/tests"
	gwconfig "sigs.k8s.io/gateway-api/conformance/utils/config"
	gwgrpc "sigs.k8s.io/gateway-api/conformance/utils/grpc"
	gwhttp "sigs.k8s.io/gateway-api/conformance/utils/http"
	"sigs.k8s.io/gateway-api/conformance/utils/roundtripper"
	gwtls "sigs.k8s.io/gateway-api/conformance/utils/tls"
)

func WaitForOutputReadiness(t *testing.T, ctx context.Context, kubeContext string, objs []unstructured.Unstructured, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for _, gc := range FilterKind(objs, "GatewayClass") {
		name := gc.GetName()
		for time.Now().Before(deadline) {
			u, err := getUnstructured(ctx, kubeContext, "gatewayclass", "", name)
			if err == nil && hasTopLevelCondition(u, "Accepted", "True") {
				break
			}
			time.Sleep(2 * time.Second)
		}
		u, err := getUnstructured(ctx, kubeContext, "gatewayclass", "", name)
		if err != nil || !hasTopLevelCondition(u, "Accepted", "True") {
			t.Fatalf("GatewayClass/%s not Accepted=True (err=%v)", name, err)
		}
	}

	for _, gw := range FilterKind(objs, "Gateway") {
		ns := gw.GetNamespace()
		if ns == "" {
			ns = "default"
		}
		name := gw.GetName()

		for time.Now().Before(deadline) {
			u, err := getUnstructured(ctx, kubeContext, "gateway", ns, name)
			if err == nil && hasTopLevelCondition(u, "Accepted", "True") && hasTopLevelCondition(u, "Programmed", "True") {
				break
			}
			time.Sleep(2 * time.Second)
		}
		u, err := getUnstructured(ctx, kubeContext, "gateway", ns, name)
		if err != nil {
			t.Fatalf("Gateway/%s get: %v", name, err)
		}
		if !hasTopLevelCondition(u, "Accepted", "True") || !hasTopLevelCondition(u, "Programmed", "True") {
			t.Fatalf("Gateway/%s not ready: need Accepted=True and Programmed=True", name)
		}
	}

	for _, hr := range FilterKind(objs, "HTTPRoute") {
		ns := hr.GetNamespace()
		if ns == "" {
			ns = "default"
		}
		name := hr.GetName()

		for time.Now().Before(deadline) {
			u, err := getUnstructured(ctx, kubeContext, "httproute", ns, name)
			if err == nil && hasRouteParentCondition(u, "Accepted", "True") && hasRouteParentCondition(u, "ResolvedRefs", "True") {
				break
			}
			time.Sleep(2 * time.Second)
		}
		u, err := getUnstructured(ctx, kubeContext, "httproute", ns, name)
		if err != nil {
			t.Fatalf("HTTPRoute/%s get: %v", name, err)
		}
		if !hasRouteParentCondition(u, "Accepted", "True") || !hasRouteParentCondition(u, "ResolvedRefs", "True") {
			t.Fatalf("HTTPRoute/%s not ready: need parents[].conditions Accepted=True and ResolvedRefs=True", name)
		}
	}

	for _, tr := range FilterKind(objs, "TLSRoute") {
		ns := tr.GetNamespace()
		if ns == "" {
			ns = "default"
		}
		name := tr.GetName()

		for time.Now().Before(deadline) {
			u, err := getUnstructured(ctx, kubeContext, "tlsroute", ns, name)
			if err == nil && hasRouteParentCondition(u, "Accepted", "True") && hasRouteParentCondition(u, "ResolvedRefs", "True") {
				break
			}
			time.Sleep(2 * time.Second)
		}
		u, err := getUnstructured(ctx, kubeContext, "tlsroute", ns, name)
		if err != nil {
			t.Fatalf("TLSRoute/%s get: %v", name, err)
		}
		if !hasRouteParentCondition(u, "Accepted", "True") || !hasRouteParentCondition(u, "ResolvedRefs", "True") {
			t.Fatalf("TLSRoute/%s not ready: need parents[].conditions Accepted=True and ResolvedRefs=True", name)
		}
	}
}

// HTTPRequestConfig contains configuration for making HTTP requests in tests.
type HTTPRequestConfig struct {
	// HostHeader is the Host header value for the request
	HostHeader string
	// Scheme is "http" or "https"
	Scheme string
	// Address is the IP address or hostname to connect to
	Address string
	// Port is the port number (empty defaults to 80 for http, 443 for https)
	Port string
	// Path is the request path (empty defaults to "/")
	Path string
	// ExpectedStatusCode is the expected HTTP status code (used if ExpectedStatusCodes is empty)
	ExpectedStatusCode int
	// ExpectedStatusCodes is a list of acceptable status codes (takes precedence over ExpectedStatusCode)
	ExpectedStatusCodes []int
	// Timeout is the maximum time to wait for the request to succeed
	Timeout time.Duration
	// Username for Basic authentication
	Username string
	// Password for Basic authentication
	Password string
	// CertPem is the TLS certificate PEM data (for TLS passthrough)
	CertPem []byte
	// KeyPem is the TLS key PEM data (for TLS passthrough)
	KeyPem []byte
	// SecretName is the name of a Kubernetes secret containing TLS certificates
	SecretName string
	// RedirectRequest specifies expected redirect details
	RedirectRequest *roundtripper.RedirectRequest
	// UnfollowRedirect if true, don't follow redirects
	UnfollowRedirect bool
	// SNI is the Server Name Indication for TLS requests
	SNI string
	// Headers is a map of custom HTTP headers to include in the request
	Headers map[string]string
}

// GRPCRequestConfig contains configuration for making gRPC requests in tests.
type GRPCRequestConfig struct {
	// Authority is the :authority pseudo-header used for route host matching.
	Authority string
	// Address is the IP address or hostname to connect to.
	Address string
	// Port is the destination port (defaults to 80 when empty).
	Port string
	// Timeout is the maximum time to wait for consistency.
	Timeout time.Duration
	// Namespace expected from the echo backend assertions (defaults to "default").
	Namespace string
	// BackendPrefix optionally requires the responding pod name to start with this prefix.
	BackendPrefix string
}

// getRoundTripper creates a DefaultRoundTripper with appropriate timeout configuration.
func getRoundTripper() roundtripper.RoundTripper {
	timeoutConfig := gwconfig.DefaultTimeoutConfig()
	timeoutConfig.RequestTimeout = 5 * time.Second
	return &roundtripper.DefaultRoundTripper{
		TimeoutConfig: timeoutConfig,
	}
}

func RequireStickySessionEventually(
	t *testing.T,
	hostHeader, scheme, address, port, path string,
	cookieName, cookieValue string,
	numRequests int,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second

	for attempt := 1; time.Now().Before(deadline); attempt++ {
		var basePod string
		ok := true

		for i := 0; i < numRequests; i++ {
			pod, code, _, err := podAndCodeFromClientWithCookie(t, hostHeader, scheme, address, port, path, cookieName, cookieValue)
			if err != nil || strings.TrimSpace(code) != "200" || pod == "" {
				ok = false
				break
			}
			if basePod == "" {
				basePod = pod
				continue
			}
			if pod != basePod {
				ok = false
				break
			}
		}

		if ok && basePod != "" {
			t.Logf("sticky session OK: cookie %s=%s consistently routed to pod %s", cookieName, cookieValue, basePod)
			return
		}

		if attempt == 1 || attempt%5 == 0 {
			t.Logf("waiting for sticky session to converge (attempt=%d cookie=%s=%s)", attempt, cookieName, cookieValue)
		}
		time.Sleep(interval)
	}

	t.Fatalf("timed out waiting for sticky session routing with cookie %s=%s", cookieName, cookieValue)
}

func RequireDifferentSessionUsuallyDifferentPod(
	t *testing.T,
	hostHeader, scheme, address, port, path string,
	cookieName string,
	cookieValueA, cookieValueB string,
	numRequests int,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second

	for time.Now().Before(deadline) {
		podA, okA := stablePodForCookie(t, hostHeader, scheme, address, port, path, cookieName, cookieValueA, numRequests)
		podB, okB := stablePodForCookie(t, hostHeader, scheme, address, port, path, cookieName, cookieValueB, numRequests)

		if okA && okB && podA != "" && podB != "" && podA != podB {
			t.Logf("different cookies mapped to different pods: %q->%s, %q->%s", cookieValueA, podA, cookieValueB, podB)
			return
		}
		time.Sleep(interval)
	}

	// Don't hard-fail if it never differs; environments can legitimately hash both cookies to same pod.
	t.Logf("note: different cookie values did not map to different pods within timeout; sticky routing still validated")
}

func stablePodForCookie(
	t *testing.T,
	hostHeader, scheme, address, port, path, cookieName, cookieValue string,
	numRequests int,
) (string, bool) {
	var basePod string
	for i := 0; i < numRequests; i++ {
		pod, code, _, err := podAndCodeFromClientWithCookie(t, hostHeader, scheme, address, port, path, cookieName, cookieValue)
		if err != nil || strings.TrimSpace(code) != "200" || pod == "" {
			return "", false
		}
		if basePod == "" {
			basePod = pod
			continue
		}
		if pod != basePod {
			return "", false
		}
	}
	return basePod, true
}

func podAndCodeFromClientWithCookie(
	t *testing.T,
	hostHeader, scheme, address, port, path string,
	cookieName, cookieValue string,
) (pod, code, out string, err error) {
	t.Helper()

	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if path == "" {
		path = "/"
	}

	gwAddr := net.JoinHostPort(address, port)

	expected := gwhttp.ExpectedResponse{
		Request: gwhttp.Request{
			Host:   hostHeader,
			Method: "GET",
			Path:   path,
		},
		Response: gwhttp.Response{StatusCode: 200},
	}

	req := gwhttp.MakeRequest(t, &expected, gwAddr, strings.ToUpper(scheme), scheme)

	if req.Headers == nil {
		req.Headers = map[string][]string{}
	}
	// Keep same downstream connection behavior controlled (optional).
	req.Headers["Connection"] = []string{"close"}
	req.Headers["X-E2E-Nonce"] = []string{fmt.Sprintf("%d", time.Now().UnixNano())}

	// Set the cookie expected by the policy.
	// HTTP Cookie header format: "name=value"
	req.Headers["Cookie"] = []string{fmt.Sprintf("%s=%s", cookieName, cookieValue)}

	rt := getRoundTripper()
	cReq, cRes, err := rt.CaptureRoundTrip(req)
	if err != nil {
		return "", "000", fmt.Sprintf("request failed: %v", err), err
	}

	if cReq != nil {
		pod = cReq.Pod
	}
	code = fmt.Sprintf("%d", cRes.StatusCode)
	out = fmt.Sprintf("Status: %d, Protocol: %s, Pod: %s", cRes.StatusCode, cRes.Protocol, pod)
	return pod, code, out, nil
}

// GetKubernetesClient creates a Kubernetes client using the kubeconfig context.
func GetKubernetesClient(kubeContext string) (client.Client, error) {
	cfg, err := ctrlconfig.GetConfigWithContext(kubeContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return cl, nil
}

// MakeHTTPRequestEventually makes an HTTP request based on the provided configuration.
// It handles regular HTTP/HTTPS requests, TLS passthrough, Basic auth, and redirects.
func MakeHTTPRequestEventually(t *testing.T, kubeContext string, cfg HTTPRequestConfig) {
	t.Helper()

	// Load TLS certificates from secret if SecretName is specified
	if cfg.SecretName != "" {
		cl, err := GetKubernetesClient(kubeContext)
		if err != nil {
			t.Fatalf("failed to create Kubernetes client: %v", err)
		}
		certPem, keyPem, err := gwtests.GetTLSSecret(cl, types.NamespacedName{Namespace: "default", Name: cfg.SecretName})
		if err != nil {
			t.Fatalf("unexpected error finding TLS secret: %v", err)
		}
		cfg.CertPem = certPem
		cfg.KeyPem = keyPem
	}

	gwAddr := net.JoinHostPort(cfg.Address, cfg.Port)

	// Build request headers
	headers := make(map[string]string)
	// Add custom headers if provided
	if cfg.Headers != nil {
		maps.Copy(headers, cfg.Headers)
	}
	var expectedRequest *gwhttp.ExpectedRequest
	if cfg.Username != "" && cfg.Password != "" {
		// Add Authorization header for basic auth
		auth := base64.StdEncoding.EncodeToString([]byte(cfg.Username + ":" + cfg.Password))
		headers["Authorization"] = "Basic " + auth
		// For basic auth, gateways strip Authorization header after validation,
		// so we expect it to be absent from the backend request
		expectedRequest = &gwhttp.ExpectedRequest{
			Request: gwhttp.Request{
				Host:   cfg.HostHeader,
				Method: "GET",
				Path:   cfg.Path,
			},
		}
	}

	// Build expected response
	expected := gwhttp.ExpectedResponse{
		Namespace:       "default",
		ExpectedRequest: expectedRequest,
		Request: gwhttp.Request{
			Host:             cfg.HostHeader,
			Method:           "GET",
			Path:             cfg.Path,
			Headers:          headers,
			UnfollowRedirect: cfg.UnfollowRedirect,
			SNI:              cfg.SNI,
		},
		RedirectRequest: cfg.RedirectRequest,
	}

	// Set expected status code(s)
	if len(cfg.ExpectedStatusCodes) > 0 {
		expected.Response.StatusCodes = cfg.ExpectedStatusCodes
	} else if cfg.ExpectedStatusCode != 0 {
		expected.Response.StatusCode = cfg.ExpectedStatusCode
	} else {
		// Default to 200 if not specified
		expected.Response.StatusCode = 200
	}

	rt := getRoundTripper()
	timeoutConfig := gwconfig.DefaultTimeoutConfig()
	timeoutConfig.MaxTimeToConsistency = cfg.Timeout
	timeoutConfig.RequiredConsecutiveSuccesses = 1

	// Use TLS utilities if certificates are provided
	if len(cfg.CertPem) > 0 && len(cfg.KeyPem) > 0 {
		sni := cfg.SNI
		if sni == "" {
			sni = cfg.HostHeader
		}
		gwtls.MakeTLSRequestAndExpectEventuallyConsistentResponse(t, rt, timeoutConfig, gwAddr, cfg.CertPem, cfg.KeyPem, sni, expected)
	} else {
		gwhttp.MakeRequestAndExpectEventuallyConsistentResponse(t, rt, timeoutConfig, gwAddr, expected)
	}
}

// MakeGRPCRequestEventually makes a gRPC request and waits for the expected response.
func MakeGRPCRequestEventually(t *testing.T, cfg GRPCRequestConfig) {
	t.Helper()

	port := cfg.Port
	if port == "" {
		port = "80"
	}
	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "default"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = time.Minute
	}

	gwAddr := net.JoinHostPort(cfg.Address, port)
	timeoutConfig := gwconfig.DefaultTimeoutConfig()
	timeoutConfig.MaxTimeToConsistency = timeout
	timeoutConfig.RequiredConsecutiveSuccesses = 1

	expected := gwgrpc.ExpectedResponse{
		EchoRequest: &pb.EchoRequest{},
		RequestMetadata: &gwgrpc.RequestMetadata{
			Authority: cfg.Authority,
		},
		Response:  gwgrpc.Response{Code: codes.OK},
		Namespace: namespace,
		Backend:   cfg.BackendPrefix,
	}

	gwgrpc.MakeRequestAndExpectEventuallyConsistentResponse(
		t,
		&gwgrpc.DefaultClient{},
		timeoutConfig,
		gwAddr,
		expected,
	)
}

func WaitForGatewayAddress(ctx context.Context, kubeContext, ns, gwName string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		u, err := getUnstructured(ctx, kubeContext, "gateway", ns, gwName)
		if err == nil {
			if addr := getGatewayStatusAddress(u); addr != "" {
				return addr, nil
			}
		}
		time.Sleep(2 * time.Second)
	}

	if _, err := getUnstructured(ctx, kubeContext, "gateway", ns, gwName); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no Gateway.status.addresses found for %s/%s", ns, gwName)
}

func WaitForServiceAddress(ctx context.Context, kubeContext, ns, name string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		u, err := getUnstructured(ctx, kubeContext, "service", ns, name)
		if err == nil {
			ings, found, _ := unstructured.NestedSlice(u.Object, "status", "loadBalancer", "ingress")
			if found && len(ings) > 0 {
				m, ok := ings[0].(map[string]any)
				if ok {
					if ip, _ := m["ip"].(string); ip != "" {
						return ip, nil
					}
					if hn, _ := m["hostname"].(string); hn != "" {
						return hn, nil
					}
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("service %s/%s has no external IP/hostname yet", ns, name)
}

// podAndCodeFromClient makes an HTTP request and returns the backend pod name and the status code.
func podAndCodeFromClient(t *testing.T, hostHeader, scheme, address, port, path string) (pod, code, out string, err error) {
	t.Helper()

	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if path == "" {
		path = "/"
	}

	gwAddr := net.JoinHostPort(address, port)

	expected := gwhttp.ExpectedResponse{
		Request: gwhttp.Request{
			Host:   hostHeader,
			Method: "GET",
			Path:   path,
		},
		Response: gwhttp.Response{StatusCode: 200},
	}

	req := gwhttp.MakeRequest(t, &expected, gwAddr, strings.ToUpper(scheme), scheme)

	// Optional but helpful to avoid “same downstream connection == same upstream host” stickiness
	// in some implementations:
	if req.Headers == nil {
		req.Headers = map[string][]string{}
	}
	req.Headers["Connection"] = []string{"close"}
	req.Headers["X-E2E-Nonce"] = []string{fmt.Sprintf("%d", time.Now().UnixNano())}

	rt := getRoundTripper()
	cReq, cRes, err := rt.CaptureRoundTrip(req)
	if err != nil {
		return "", "000", fmt.Sprintf("request failed: %v", err), err
	}

	if cReq != nil {
		pod = cReq.Pod
	}
	code = fmt.Sprintf("%d", cRes.StatusCode)
	out = fmt.Sprintf("Status: %d, Protocol: %s, Pod: %s", cRes.StatusCode, cRes.Protocol, pod)
	return pod, code, out, nil
}

// pathAndCodeFromClient makes an HTTP request and returns the backend echoed path and status code.
// The echoed path comes from the conformance CapturedRequest (decoded from echo-backend JSON).
func pathAndCodeFromClient(t *testing.T, hostHeader, scheme, address, port, path string) (echoPath, code, out string, err error) {
	t.Helper()

	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if path == "" {
		path = "/"
	}

	gwAddr := net.JoinHostPort(address, port)

	expected := gwhttp.ExpectedResponse{
		Request: gwhttp.Request{
			Host:   hostHeader,
			Method: "GET",
			Path:   path,
		},
		Response: gwhttp.Response{StatusCode: 200},
	}

	req := gwhttp.MakeRequest(t, &expected, gwAddr, strings.ToUpper(scheme), scheme)

	// Reduce chances of connection-based stickiness.
	if req.Headers == nil {
		req.Headers = map[string][]string{}
	}
	req.Headers["Connection"] = []string{"close"}
	req.Headers["X-E2E-Nonce"] = []string{fmt.Sprintf("%d", time.Now().UnixNano())}

	rt := getRoundTripper()
	cReq, cRes, err := rt.CaptureRoundTrip(req)
	if err != nil {
		return "", "000", fmt.Sprintf("request failed: %v", err), err
	}

	if cReq != nil {
		echoPath = cReq.Path
	}
	code = fmt.Sprintf("%d", cRes.StatusCode)
	out = fmt.Sprintf("Status: %d, Protocol: %s, EchoPath: %s", cRes.StatusCode, cRes.Protocol, echoPath)
	return echoPath, code, out, nil
}

// containsInt checks if a slice of ints contains a specific int.
func containsInt(haystack []int, needle int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// RequireResponseHeaderEventually polls until the response header matches the expectation.
func RequireResponseHeaderEventually(
	t *testing.T,
	cfg HTTPRequestConfig,
	headerName string,
	wantPresent bool,
	wantValue string,
) {
	t.Helper()

	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}
	if cfg.Path == "" {
		cfg.Path = "/"
	}

	port := cfg.Port
	if port == "" {
		if strings.EqualFold(cfg.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	gwAddr := net.JoinHostPort(cfg.Address, port)

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = time.Minute
	}

	expectedCodes := cfg.ExpectedStatusCodes
	if len(expectedCodes) == 0 {
		code := cfg.ExpectedStatusCode
		if code == 0 {
			code = 200
		}
		expectedCodes = []int{code}
	}

	rt := getRoundTripper()

	deadline := time.Now().Add(timeout)
	var (
		lastStatus int
		lastVals   []string
		lastErr    error
	)

	for time.Now().Before(deadline) {
		// Build an expected request similar to MakeHTTPRequestEventually.
		expected := gwhttp.ExpectedResponse{
			Request: gwhttp.Request{
				Host:             cfg.HostHeader,
				Method:           "GET",
				Path:             cfg.Path,
				Headers:          cfg.Headers,
				UnfollowRedirect: cfg.UnfollowRedirect,
				SNI:              cfg.SNI,
			},
			RedirectRequest: cfg.RedirectRequest,
		}

		if len(expectedCodes) > 1 {
			expected.Response.StatusCodes = expectedCodes
		} else {
			expected.Response.StatusCode = expectedCodes[0]
		}

		for k, v := range cfg.Headers {
			expected.Request.Headers[k] = v
		}

		// Avoid keep-alives / caches affecting header assertions across retries.
		expected.Request.Headers["Connection"] = "close"
		expected.Request.Headers["X-E2E-Nonce"] = fmt.Sprintf("%d", time.Now().UnixNano())

		req := gwhttp.MakeRequest(t, &expected, gwAddr, strings.ToUpper(cfg.Scheme), cfg.Scheme)
		_, cRes, err := rt.CaptureRoundTrip(req)
		if err != nil || cRes == nil {
			lastErr = err
			time.Sleep(250 * time.Millisecond)
			continue
		}

		lastStatus = cRes.StatusCode
		if !containsInt(expectedCodes, cRes.StatusCode) {
			time.Sleep(250 * time.Millisecond)
			continue
		}

		canon := textproto.CanonicalMIMEHeaderKey(headerName)
		lastVals = cRes.Headers[canon]

		if wantPresent {
			if len(lastVals) == 0 {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			if wantValue != "" {
				ok := false
				for _, v := range lastVals {
					if v == wantValue {
						ok = true
						break
					}
				}
				if !ok {
					time.Sleep(250 * time.Millisecond)
					continue
				}
			}
			return
		}

		// want absent
		if len(lastVals) != 0 {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		return
	}

	t.Fatalf(
		"timed out waiting for response header assertion: header=%q wantPresent=%v wantValue=%q lastStatus=%d lastHeaderVals=%v lastErr=%v",
		headerName, wantPresent, wantValue, lastStatus, lastVals, lastErr,
	)
}

func RequireEchoedPathEventually(
	t *testing.T,
	hostHeader, scheme, address, port, requestPath, expectedEchoPath string,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second

	var lastOut string
	var lastErr error
	var lastCode string
	var lastEchoPath string

	for attempt := 1; time.Now().Before(deadline); attempt++ {
		echoPath, code, out, err := pathAndCodeFromClient(t, hostHeader, scheme, address, port, requestPath)
		lastEchoPath, lastCode, lastOut, lastErr = echoPath, code, out, err

		if err == nil && strings.TrimSpace(code) == "200" && echoPath == expectedEchoPath {
			return
		}

		if attempt == 1 || attempt%10 == 0 {
			t.Logf("waiting for echoed path %q (attempt=%d host=%s scheme=%s address=%s port=%s reqPath=%s): gotPath=%q code=%q err=%v out=%s",
				expectedEchoPath, attempt, hostHeader, scheme, address, port, requestPath, echoPath, strings.TrimSpace(code), err, out)
		}
		time.Sleep(interval)
	}

	t.Fatalf("timed out waiting for echoed path %q (request %q). lastEchoPath=%q lastCode=%q lastErr=%v lastOut=%s",
		expectedEchoPath, requestPath, lastEchoPath, strings.TrimSpace(lastCode), lastErr, lastOut)
}

func RequireLoadBalancedAcrossPodsEventually(
	t *testing.T,
	hostHeader, scheme, address, port, path string,
	wantDistinctPods int,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second

	for attempt := 1; time.Now().Before(deadline); attempt++ {
		seen := map[string]int{}
		var lastOut string
		var lastErr error
		var lastCode string

		// Enough samples that “3 replicas but I only saw 1–2 by chance” is very unlikely.
		for i := 0; i < 60; i++ {
			pod, code, out, err := podAndCodeFromClient(t, hostHeader, scheme, address, port, path)
			lastOut, lastErr, lastCode = out, err, code
			if err == nil && strings.TrimSpace(code) == "200" && pod != "" {
				seen[pod]++
			}
		}

		if len(seen) >= wantDistinctPods {
			t.Logf("load balancing OK (distinctPods=%d): %v", len(seen), seen)
			return
		}

		if attempt == 1 || attempt%5 == 0 {
			t.Logf("waiting for load balancing across %d pods (attempt=%d): seen=%v lastCode=%s lastErr=%v lastOut=%s",
				wantDistinctPods, attempt, seen, strings.TrimSpace(lastCode), lastErr, lastOut)
		}
		time.Sleep(interval)
	}

	t.Fatalf("timed out waiting to observe %d distinct backend pods", wantDistinctPods)
}
