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

package agentgateway

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	common "github.com/kgateway-dev/ingress2gateway/test/e2e/emitters/common"
	testutils "github.com/kgateway-dev/ingress2gateway/test/e2e/utils"
)

var (
	e2eSetupComplete  bool
	kubeContext       = common.KubeContext
	defaultHostHeader = common.DefaultHostHeader
)

func TestMain(m *testing.M) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	common.MustHaveE2EBinaries()

	ctx := context.Background()
	kubeContext = common.KubeContext

	kindClusterName := common.KindClusterName

	common.EnsureKindCluster(ctx, kindClusterName)
	common.ExportKubeconfig(ctx, kindClusterName)
	common.InstallPrereqs(ctx, kubeContext, common.PrereqConfig{
		MetalLBVersion:      common.DefaultMetalLBVersion,
		GatewayAPIVersion:   common.DefaultGatewayAPIVersion,
		IngressNginxVersion: common.DefaultIngressNginxVersion,
	})

	// Install agentgateway (chart version defaults to the module version in go.mod).
	installAgentgateway(ctx, kubeContext)

	// Shared backend test servers (kept across subtests).
	common.InstallSharedBackends(ctx, kubeContext, common.DefaultEchoImage)

	e2eSetupComplete = true

	// Run tests
	code := m.Run()

	// Give stdout/stderr a moment to flush in some CI environments.
	time.Sleep(100 * time.Millisecond)

	// Cleanup kind cluster if needed.
	common.CleanupKindCluster(ctx, kindClusterName, common.KeepCluster)

	os.Exit(code)
}

// e2eTestSetup handles common setup for e2e tests and returns the context, gateway address, host, and ingress address.
// The caller is responsible for cleanup and test-specific validation.
func e2eTestSetup(t *testing.T, inputFile, outputFile string) (context.Context, string, string, string, string) {
	if !e2eSetupComplete {
		t.Fatalf("e2e setup did not complete")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	root, err := testutils.ModuleRoot(ctx)
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}

	inputDir := filepath.Join(root, "test/e2e/emitters/agentgateway/testdata/input")
	outputDir := filepath.Join(root, "test/e2e/emitters/agentgateway/testdata/output")

	inPath := filepath.Join(inputDir, inputFile)
	outPath := filepath.Join(outputDir, outputFile)

	// Apply input Ingress YAML.
	testutils.MustKubectl(ctx, kubeContext, "apply", "-f", inPath)

	// Ensure cleanup of per-test resources (but keep curl + echo).
	// Note: generatedOutPath cleanup is handled separately after it's created.
	t.Cleanup(func() {
		if _, delErr := testutils.Kubectl(ctx, kubeContext, "delete", "-f", inPath, "--ignore-not-found=true", "--wait=true", "--timeout=2m"); delErr != nil {
			t.Logf("failed to delete input: %v", delErr)
		}
	})

	ingObjs, err := testutils.DecodeObjects(inPath)
	if err != nil {
		t.Fatalf("decode input objects: %v", err)
	}
	ingresses := testutils.FilterKind(ingObjs, "Ingress")
	if len(ingresses) == 0 {
		t.Fatalf("input %s had no Ingress objects", inPath)
	}

	// Get ingress-nginx-controller Service IP
	ingressIP, err := testutils.GetIngressNginxControllerAddress(ctx, kubeContext)
	if err != nil {
		t.Fatalf("get ingress-nginx-controller service address: %v", err)
	}

	// Extract host header from Ingress resources.
	var hostHeader string
	for _, ing := range ingresses {
		if h, _ := testutils.FirstIngressHost(ing); h != "" {
			hostHeader = h
			break
		}
	}
	if hostHeader == "" {
		hostHeader = defaultHostHeader
	}

	// Verify expected output file exists for comparison.
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output file missing: %s (%v)", outPath, err)
	}

	// Run ingress2gateway to generate output from input, compare with expected output,
	// and get the path to the generated output file.
	generatedOutPath, err := testutils.CompareAndGenerateOutput(ctx, t, "agentgateway", root, inPath, outPath)
	if err != nil {
		t.Fatalf("failed to generate and compare output: %v", err)
	}

	// Cleanup: delete generated resources, then remove the temp file.
	// Cleanup functions run in reverse order, so this will run after resource deletion.
	t.Cleanup(func() {
		if _, delErr := testutils.Kubectl(ctx, kubeContext, "delete", "-f", generatedOutPath, "--ignore-not-found=true", "--wait=true", "--timeout=2m"); delErr != nil {
			t.Logf("failed to delete generated output resources: %v", delErr)
		}
		if err := os.Remove(generatedOutPath); err != nil {
			t.Logf("failed to remove generated output temp file %q: %v", generatedOutPath, err)
		}
	})

	// Apply the generated ingress2gateway output YAML.
	testutils.MustKubectl(ctx, kubeContext, "apply", "-f", generatedOutPath)

	outObjs, err := testutils.DecodeObjects(generatedOutPath)
	if err != nil {
		t.Fatalf("decode output objects: %v", err)
	}

	// Check expected status conditions (GatewayClass, Gateway, HTTPRoute, etc.).
	testutils.WaitForOutputReadiness(t, ctx, kubeContext, outObjs, 1*time.Minute)

	// Get Gateway address.
	gws := testutils.FilterKind(outObjs, "Gateway")
	if len(gws) == 0 {
		t.Fatalf("generated output had no Gateway objects")
	}
	gw := gws[0]
	gwNS := gw.GetNamespace()
	if gwNS == "" {
		gwNS = "default"
	}
	gwName := gw.GetName()

	gwAddr, err := testutils.WaitForGatewayAddress(ctx, kubeContext, gwNS, gwName, 1*time.Minute)
	if err != nil {
		t.Fatalf("gateway address: %v", err)
	}

	// Prefer HTTPRoute or TLSRoute hostnames only when the input didn't specify any hosts.
	host := hostHeader
	if hostHeader == defaultHostHeader {
		if hr := testutils.FirstRouteHost(outObjs); hr != "" {
			host = hr
		}
	}

	return ctx, gwAddr, host, hostHeader, ingressIP
}

func TestBasic(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "basic.yaml", "basic.yaml")

	// Test HTTP connectivity via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Test HTTP connectivity via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})
}

func TestBackendProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	// Use a dedicated gRPC backend so the shared echo backend can remain HTTP for other e2e cases.
	testutils.ApplyGRPCEchoBackend(ctx, kubeContext)
	t.Cleanup(func() {
		if _, err := testutils.Kubectl(ctx, kubeContext, "-n", "default", "delete", "deploy/echo-backend-grpc", "svc/echo-backend-grpc", "--ignore-not-found=true", "--wait=true", "--timeout=2m"); err != nil {
			t.Logf("failed to delete gRPC backend resources: %v", err)
		}
	})

	_, gwAddr, host, _, _ := e2eTestSetup(t, "backend_protocol.yaml", "backend_protocol.yaml")

	// Validate end-to-end gRPC traffic via Gateway when backend-protocol is projected.
	testutils.MakeGRPCRequestEventually(t, testutils.GRPCRequestConfig{
		Authority:     host,
		Address:       gwAddr,
		Port:          "80",
		Timeout:       1 * time.Minute,
		Namespace:     "default",
		BackendPrefix: "echo-backend-grpc",
	})
}

func TestLoadBalance(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "load_balance.yaml", "load_balance.yaml")

	// Test HTTP connectivity via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Test HTTP connectivity via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Assert we actually see all 3 backends.
	testutils.RequireLoadBalancedAcrossPodsEventually(t, host, "http", gwAddr, "80", "/", 3, 1*time.Minute)
}

func TestFrontendTLS(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "frontend_tls.yaml", "frontend_tls.yaml")

	// Validate ingress-nginx behavior for baseline parity.
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Validate agentgateway output is accepted and routes traffic.
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})
}

func TestRateLimit(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "rate_limit.yaml", "rate_limit.yaml")

	// Test HTTP connectivity via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Test HTTP connectivity via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})
}

func TestTimeouts(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "timeouts.yaml", "timeouts.yaml")

	// Test HTTP connectivity via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Test HTTP connectivity via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})
}

func TestBasicAuth(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "basic_auth.yaml", "basic_auth.yaml")

	username := "user"
	password := "password"

	// Test unauthenticated request → expect 401 via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 401,
		Timeout:            1 * time.Minute,
	})

	// Test unauthenticated request → expect 401 via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 401,
		Timeout:            1 * time.Minute,
	})

	// Test authenticated request with valid credentials → expect 200 via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
		Username:           username,
		Password:           password,
	})

	// Test authenticated request with valid credentials → expect 200 via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
		Username:           username,
		Password:           password,
	})

	// Test authenticated request with invalid credentials → expect 401 via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 401,
		Timeout:            1 * time.Minute,
		Username:           username,
		Password:           "wrongpassword",
	})
}

func TestExternalAuth(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "external_auth.yaml", "external_auth.yaml")

	// Test unauthenticated request → expect 401 via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 401,
		Timeout:            1 * time.Minute,
	})

	// Test authenticated request with valid token → expect 200 via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
		},
	})

	// Test unauthenticated request → expect 401 via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 401,
		Timeout:            1 * time.Minute,
	})

	// Test authenticated request with valid token → expect 200 via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
		},
	})

	// Test authenticated request with invalid token → expect 401 via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 401,
		Timeout:            1 * time.Minute,
		Headers: map[string]string{
			"Authorization": "Bearer invalid-token",
		},
	})
}

func TestCORS(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "cors.yaml", "cors.yaml")

	// Test HTTP connectivity via Ingress
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         ingressHostHeader,
		Scheme:             "http",
		Address:            ingressIP,
		Port:               "",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Test HTTP connectivity via Gateway
	testutils.MakeHTTPRequestEventually(t, kubeContext, testutils.HTTPRequestConfig{
		HostHeader:         host,
		Scheme:             "http",
		Address:            gwAddr,
		Port:               "80",
		Path:               "/",
		ExpectedStatusCode: 200,
		Timeout:            1 * time.Minute,
	})

	// Negative coverage:
	// - ensure upstream CORS headers are stripped
	// - ensure only configured allowOrigins are reflected back
	//
	// We force the upstream echo backend to emit a permissive ACAO header using
	// X-Echo-Set-Header (echo server feature) and then assert the gateway behavior.
	base := testutils.HTTPRequestConfig{
		Scheme:              "http",
		Address:             gwAddr,
		HostHeader:          "cors.localdev.me",
		Port:                "80",
		Path:                "/get",
		ExpectedStatusCodes: []int{200},
		Timeout:             time.Minute,
	}

	// No Origin header: should NOT emit ACAO (and should strip upstream ACAO).
	{
		cfg := base
		cfg.Headers = map[string]string{
			"X-Echo-Set-Header": "Access-Control-Allow-Origin:*",
		}
		testutils.RequireResponseHeaderEventually(t, cfg, "Access-Control-Allow-Origin", false, "")
	}

	// Disallowed Origin: should NOT emit ACAO (and should strip upstream ACAO).
	{
		cfg := base
		cfg.Headers = map[string]string{
			"Origin":            "https://evil.com",
			"X-Echo-Set-Header": "Access-Control-Allow-Origin:*",
		}
		testutils.RequireResponseHeaderEventually(t, cfg, "Access-Control-Allow-Origin", false, "")
	}

	// Allowed Origin: should emit ACAO:<origin> (and must NOT leak upstream '*').
	{
		cfg := base
		cfg.Headers = map[string]string{
			"Origin":            "https://example.com",
			"X-Echo-Set-Header": "Access-Control-Allow-Origin:*",
		}
		testutils.RequireResponseHeaderEventually(t, cfg, "Access-Control-Allow-Origin", true, "https://example.com")
	}
}

func TestRewriteTarget(t *testing.T) {
	_, gwAddr, host, ingressHostHeader, ingressIP := e2eTestSetup(t, "rewrite_target.yaml", "rewrite_target.yaml")

	// Must match "test/e2e/emitters/agentgateway/testdata/output/rewrite_target.yaml".
	reqPath := "/before/rewrite"
	wantPath := "/after/rewrite"

	// Validate behavior through Ingress (ingress-nginx)
	testutils.RequireEchoedPathEventually(t, ingressHostHeader, "http", ingressIP, "", reqPath, wantPath, 1*time.Minute)

	// Validate behavior through Gateway (agentgateway + generated AgentgatewayPolicy)
	testutils.RequireEchoedPathEventually(t, host, "http", gwAddr, "80", reqPath, wantPath, 1*time.Minute)
}
