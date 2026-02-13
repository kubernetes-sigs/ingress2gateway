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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// A prefix for all namespaces used in the e2e tests.
const e2ePrefix = "i2gw"
const DummyAppName1 = "dummy-app1"
const DummyAppName2 = "dummy-app2"

type testCase struct {
	ingresses              []*networkingv1.Ingress
	secrets                []*corev1.Secret
	providers              []string
	providerFlags          map[string]map[string]string
	gatewayImplementations []string
	allowExperimentalGWAPI bool
	verifiers              map[string][]verifier
}

func runTestCase(t *testing.T, tc *testCase) {
	t.Parallel()

	if len(tc.providers) == 0 {
		t.Fatal("At least one provider must be specified")
	}

	if len(tc.gatewayImplementations) == 0 {
		t.Fatal("gatewayImplementations must be specified")
	}

	ctx := t.Context()

	// We deliberately avoid setting a default kubeconfig so that we don't accidentally create e2e
	// resources on a production cluster.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Fatal("Environment variable KUBECONFIG must be set")
	}

	skipCleanup := os.Getenv("SKIP_CLEANUP") == "1"

	k8sClient, err := newClientFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	gwClient, err := newGatewayClientFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	apiextensionsClient, err := newAPIExtensionsClientFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	restConfig, err := newRestConfigFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	// Generate a random prefix to ensure unique namespaces and hostnames for each test case.
	randPrefix, err := randString()
	require.NoError(t, err)
	nsPrefix := fmt.Sprintf("%s-%s", e2ePrefix, randPrefix)

	appNS := fmt.Sprintf("%s-app", nsPrefix)
	cleanupNS, err := createNamespace(ctx, t, k8sClient, appNS, skipCleanup)
	require.NoError(t, err)
	t.Cleanup(cleanupNS)

	crdResource := globalResourceManager.acquire("gateway-api-crds", func() (cleanupFunc, error) {
		return deployCRDs(ctx, t, apiextensionsClient, skipCleanup)
	})
	t.Cleanup(func() {
		<-crdResource.cleanup()
	})
	require.NoError(t, crdResource.wait(), "Gateway API CRDs installation failed")

	resources := deployProviders(ctx, t, k8sClient, kubeconfig, tc.providers, tc.gatewayImplementations, skipCleanup)

	for _, g := range tc.gatewayImplementations {
		in
		gwImpl := deployGatewayImplementation(ctx, t, k8sClient, gwClient, kubeconfig, g, skipCleanup)
		resources = append(resources, gwImpl)
	}

	// Clean up all providers and the GWAPI implementation in parallel.
	t.Cleanup(func() {
		var doneChans []<-chan struct{}
		for _, r := range resources {
			doneChans = append(doneChans, r.cleanup())
		}
		for _, ch := range doneChans {
			<-ch
		}
	})

	for _, r := range resources {
		require.NoError(t, r.wait(), "resource installation failed: %s", r.name)
	}

	cleanupDummyApp1, err := deployDummyApp(ctx, t, k8sClient, DummyAppName1, appNS, skipCleanup)
	require.NoError(t, err, "creating %s", DummyAppName1)
	t.Cleanup(cleanupDummyApp1)
	cleanupDummyApp2, err := deployDummyApp(ctx, t, k8sClient, DummyAppName2, appNS, skipCleanup)
	require.NoError(t, err, "creating %s", DummyAppName2)
	t.Cleanup(cleanupDummyApp2)

	// Populate ingress Host field if not specified in the test case.
	for _, ing := range tc.ingresses {
		for i := range ing.Spec.Rules {
			if ing.Spec.Rules[i].Host == "" {
				ing.Spec.Rules[i].Host = fmt.Sprintf("%s.%s.%s.test", ing.Name, randPrefix, e2ePrefix)
			}
		}
	}

	if len(tc.secrets) > 0 {
		cleanupSecrets, secretsErr := createSecrets(ctx, t, k8sClient, appNS, tc.secrets, skipCleanup)
		require.NoError(t, secretsErr, "creating secrets")
		t.Cleanup(cleanupSecrets)
	}

	cleanupIngresses, err := createIngresses(ctx, t, k8sClient, appNS, tc.ingresses, skipCleanup)
	require.NoError(t, err)
	t.Cleanup(cleanupIngresses)

	// Set up port forwarding to the ingress controllers for verification.
	ingressPortForwarders, ingressAddresses := setUpIngressPortForwarding(
		ctx,
		t,
		k8sClient,
		restConfig,
		tc.providers,
		testCaseNeedsHTTPS(tc),
	)
	t.Cleanup(func() {
		for _, pf := range ingressPortForwarders {
			pf.stop()
		}
	})

	verifyIngresses(ctx, t, tc, ingressAddresses)

	for _, g := range tc.gatewayImplementations {
		// Run the ingress2gateway binary to convert ingresses to Gateway API resources.
		res := runI2GW(ctx, t, kubeconfig, appNS, tc.providers, tc.providerFlags, tc.allowExperimentalGWAPI)

		// TODO: Hack! Force correct gateway class since i2gw doesn't seem to infer that from the
		// ingress at the moment.
		for _, r := range res {
			for k, v := range r.Gateways {
				v.Spec.GatewayClassName = gwapiv1.ObjectName(g)
				r.Gateways[k] = v
			}
		}

		cleanupGatewayResources, err := createGatewayResources(ctx, t, gwClient, appNS, res, skipCleanup)
		require.NoError(t, err, "creating gateway resources")
		t.Cleanup(cleanupGatewayResources)

		// Set up port forwarding to each gateway for verification.
		gatewayPortForwarders, gwAddresses := setUpGatewayPortForwarding(
			ctx,
			t,
			k8sClient,
			restConfig,
			getGateways(res),
			appNS,
			g,
			testCaseNeedsHTTPS(tc),
		)
		t.Cleanup(func() {
			for _, pf := range gatewayPortForwarders {
				pf.stop()
			}
		})

		t.Run(fmt.Sprintf("to %s", g), func(t *testing.T) {
			verifyGatewayResources(ctx, t, tc, gwAddresses)
		})
	}
}

func deployProviders(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
	kubeconfig string,
	providers []string,
	gwImpls []string,
	skipCleanup bool,
) []resource {
	var resources []resource

	for _, p := range providers {
		var r resource
		switch p {
		case ingressnginx.Name:
			ns := fmt.Sprintf("%s-ingress-nginx", e2ePrefix)
			r = globalResourceManager.acquire(ingressnginx.Name, func() (cleanupFunc, error) {
				return deployIngressNginx(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
			})
		case kong.Name:
			// If Kong is also the gateway implementation, skip deploying Kong ingress separately.
			// The gateway deployment will handle both Ingress and Gateway API.
			if slices.Contains(gwImpls, kong.Name) {
				continue
			}
			ns := fmt.Sprintf("%s-kong", e2ePrefix)
			r = globalResourceManager.acquire(kong.Name, func() (cleanupFunc, error) {
				return deployKongIngress(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
			})
		default:
			t.Fatalf("Unknown ingress provider: %s", p)
		}
		resources = append(resources, r)
	}

	return resources
}

func deployGatewayImplementation(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
	gwClient *gwclientset.Clientset,
	kubeconfig string,
	gwImpl string,
	skipCleanup bool,
) resource {
	var r resource

	switch gwImpl {
	case istio.ProviderName:
		ns := fmt.Sprintf("%s-istio-system", e2ePrefix)
		r = globalResourceManager.acquire(istio.ProviderName, func() (cleanupFunc, error) {
			return deployGatewayAPIIstio(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
		})
	case kong.Name:
		ns := fmt.Sprintf("%s-kong", e2ePrefix)
		r = globalResourceManager.acquire(kong.Name, func() (cleanupFunc, error) {
			return deployGatewayAPIKong(ctx, t, k8sClient, gwClient, kubeconfig, ns, skipCleanup)
		})
	default:
		t.Fatalf("Unknown gateway implementation: %s", gwImpl)
	}

	return r
}

// Sets up port forwarders for all ingress providers. Returns the resulting portForwarders and a
// map of ingress class to address.
func setUpIngressPortForwarding(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
	restConfig *rest.Config,
	providers []string,
	useHTTPS bool,
) ([]*portForwarder, map[string]addresses) {
	var pfs []*portForwarder
	pfAddresses := make(map[string]addresses)

	for _, p := range providers {
		var ingressClass string
		var svc *corev1.Service
		var ingressNS string

		switch p {
		case ingressnginx.Name:
			ingressNS = fmt.Sprintf("%s-ingress-nginx", e2ePrefix)
			ingressClass = ingressnginx.NginxIngressClass
		case kong.Name:
			// Kong uses the same namespace for both ingress and gateway when both are enabled.
			ingressNS = fmt.Sprintf("%s-kong", e2ePrefix)
			ingressClass = kong.KongIngressClass
		default:
			t.Fatalf("Unknown ingress provider: %s", p)
		}

		svc, err := findIngressControllerService(ctx, k8sClient, ingressNS, p)
		require.NoError(t, err, "finding %s service", p)

		t.Logf("Waiting for ingress controller %s service %s/%s to have ready pods", p, svc.Namespace, svc.Name)
		err = waitForServiceReady(ctx, k8sClient, svc.Namespace, svc.Name)
		require.NoError(t, err, "waiting for %s service to be ready", p)

		pf, addr, err := startPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 80)
		require.NoError(t, err, "starting port forward to %s", p)
		pfs = append(pfs, pf)
		pfAddresses[ingressClass] = addresses{http: addr}
		if useHTTPS {
			httpsPf, httpsAddr, err := startPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 443)
			require.NoError(t, err, "starting https port forward to %s", p)
			pfs = append(pfs, httpsPf)
			pfAddresses[ingressClass] = addresses{http: addr, https: httpsAddr}
		}
		t.Logf("Port forwarding ingress controller %s via %s", p, addr)
	}

	return pfs, pfAddresses
}

func setUpGatewayPortForwarding(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
	restConfig *rest.Config,
	gateways map[types.NamespacedName]gwapiv1.Gateway,
	appNS string,
	gwImpl string,
	useHTTPS bool,
) ([]*portForwarder, map[string]addresses) {
	var pfs []*portForwarder
	pfAddresses := make(map[string]addresses)

	for gwName, gw := range gateways {
		ns := gw.Namespace
		if ns == "" {
			ns = appNS
		}

		// Find the service created by the gateway controller.
		// Kong in unmanaged mode doesn't create per-gateway services - all gateways share the same
		// proxy service in the Kong namespace.
		var svc *corev1.Service
		var err error
		if gwImpl == kong.Name {
			kongNS := fmt.Sprintf("%s-kong", e2ePrefix)
			svc, err = findIngressControllerService(ctx, k8sClient, kongNS, kong.Name)
		} else {
			svc, err = findGatewayService(ctx, t, k8sClient, ns, gw.Name)
		}
		require.NoError(t, err, "finding gateway service for %s", gwName)

		// Wait for at least one pod to be ready before port forwarding.
		t.Logf("Waiting for gateway %s service %s/%s to have ready pods", gwName, svc.Namespace, svc.Name)
		err = waitForServiceReady(ctx, k8sClient, svc.Namespace, svc.Name)
		require.NoError(t, err, "waiting for gateway service %s/%s to be ready", svc.Namespace, svc.Name)

		// Start port forward to the gateway service.
		pf, addr, err := startPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 80)
		require.NoError(t, err, "starting port forward for gateway %s", gwName)

		pfs = append(pfs, pf)
		pfAddresses[gwName.Name] = addresses{http: addr}
		if useHTTPS {
			httpsPf, httpsAddr, err := startPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 443)
			require.NoError(t, err, "starting https port forward for gateway %s", gwName)
			pfs = append(pfs, httpsPf)
			pfAddresses[gwName.Name] = addresses{http: addr, https: httpsAddr}
		}
		t.Logf("Port forwarding gateway %s via %s", gwName, addr)
	}

	return pfs, pfAddresses
}

func verifyIngresses(ctx context.Context, t *testing.T, tc *testCase, ingressAddresses map[string]addresses) {
	ingressByName := make(map[string]*networkingv1.Ingress, len(tc.ingresses))
	for _, ing := range tc.ingresses {
		ingressByName[ing.Name] = ing
	}

	for ingressName, verifiers := range tc.verifiers {
		ingress, ok := ingressByName[ingressName]
		require.True(t, ok, "ingress %s not found in test case", ingressName)

		ingressClass := common.GetIngressClass(*ingress)
		require.NotEmpty(t, ingressClass, "ingress %s has no ingress class", ingressName)

		addr, ok := ingressAddresses[ingressClass]
		require.True(t, ok, "no address found for ingress class %s", ingressClass)

		for _, v := range verifiers {
			err := retry(ctx, t, retryConfig{maxAttempts: 60, delay: 1 * time.Second},
				func(attempt int, maxAttempts int, err error) string {
					return fmt.Sprintf("Verifying ingress %s (attempt %d/%d): %v", ingressName, attempt, maxAttempts, err)
				},
				func() error {
					return v.verify(ctx, t, addr, ingress)
				},
			)
			require.NoError(t, err, "ingress verification failed")
		}
	}
}

func verifyGatewayResources(ctx context.Context, t *testing.T, tc *testCase, gwAddresses map[string]addresses) {
	for ingressName, verifiers := range tc.verifiers {
		// Find the ingress to determine the expected gateway name.
		var ingress *networkingv1.Ingress
		for _, ing := range tc.ingresses {
			if ing.Name == ingressName {
				ingress = ing
				break
			}
		}
		if ingress == nil {
			t.Fatalf("Ingress %s not found in test case", ingressName)
		}

		// Gateway name is derived from ingress class.
		gwName := common.GetIngressClass(*ingress)
		if gwName == "" {
			t.Fatalf("Ingress %s has no ingress class", ingressName)
		}

		addr, ok := gwAddresses[gwName]
		require.True(t, ok, "gateway %s not found in addresses", gwName)

		for _, v := range verifiers {
			err := retry(ctx, t, retryConfig{maxAttempts: 60, delay: 1 * time.Second},
				func(attempt int, maxAttempts int, err error) string {
					return fmt.Sprintf("Verifying gateway %s (attempt %d/%d): %v", gwName, attempt, maxAttempts, err)
				},
				func() error {
					return v.verify(ctx, t, addr, ingress)
				},
			)
			require.NoError(t, err, "gateway verification failed")
		}
	}
}

func testCaseNeedsHTTPS(tc *testCase) bool {
	for _, verifiers := range tc.verifiers {
		for _, v := range verifiers {
			if hv, ok := v.(*httpRequestVerifier); ok && hv.useTLS {
				return true
			}
		}
	}
	return false
}

// Executes the ingress2gateway binary and returns the parsed Gateway API resources.
func runI2GW(
	ctx context.Context,
	t *testing.T,
	kubeconfig string,
	namespace string,
	providers []string,
	providerFlags map[string]map[string]string,
	allowExperimental bool,
) []i2gw.GatewayResources {
	binaryPath := os.Getenv("I2GW_BINARY_PATH")
	require.NotEmpty(t, binaryPath, "environment variable I2GW_BINARY_PATH not set")

	args := []string{
		"print",
		"--kubeconfig", kubeconfig,
		"--namespace", namespace,
		"--providers", strings.Join(providers, ","),
	}
	if allowExperimental {
		args = append(args, "--allow-experimental-gw-api")
	}

	// Add provider-specific flags.
	for provider, flags := range providerFlags {
		for flagName, flagValue := range flags {
			args = append(args, fmt.Sprintf("--%s-%s", provider, flagName), flagValue)
		}
	}

	t.Logf("Running ingress2gateway: %s %v", binaryPath, args)

	// #nosec G204 -- binaryPath is from trusted env var, args are constructed internally
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.NoError(t, err, "ingress2gateway run failed\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())

	// Log any notifications from stderr.
	if stderr.Len() > 0 {
		t.Log("Got stderr from ingress2gateway:\n", stderr.String())
	}

	return parseYAMLOutput(t, stdout.Bytes())
}

// Parses the YAML output from the ingress2gateway binary into Gateway API resources.
func parseYAMLOutput(t *testing.T, data []byte) []i2gw.GatewayResources {
	res := i2gw.GatewayResources{
		Gateways:           make(map[types.NamespacedName]gwapiv1.Gateway),
		GatewayClasses:     make(map[types.NamespacedName]gwapiv1.GatewayClass),
		HTTPRoutes:         make(map[types.NamespacedName]gwapiv1.HTTPRoute),
		GRPCRoutes:         make(map[types.NamespacedName]gwapiv1.GRPCRoute),
		TLSRoutes:          make(map[types.NamespacedName]v1alpha2.TLSRoute),
		TCPRoutes:          make(map[types.NamespacedName]v1alpha2.TCPRoute),
		UDPRoutes:          make(map[types.NamespacedName]v1alpha2.UDPRoute),
		BackendTLSPolicies: make(map[types.NamespacedName]gwapiv1.BackendTLSPolicy),
		ReferenceGrants:    make(map[types.NamespacedName]v1beta1.ReferenceGrant),
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bufio.NewReader(bytes.NewReader(data)), 4096)

	for {
		var rawObj map[string]interface{}
		if err := decoder.Decode(&rawObj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("Failed to decode YAML: %v", err)
		}

		if rawObj == nil {
			continue
		}

		apiVersion, _ := rawObj["apiVersion"].(string)
		kind, _ := rawObj["kind"].(string)
		metadata, _ := rawObj["metadata"].(map[string]interface{})
		name, _ := metadata["name"].(string)
		namespace, _ := metadata["namespace"].(string)

		nn := types.NamespacedName{Namespace: namespace, Name: name}

		// Re-encode the object to JSON bytes for proper unmarshaling.
		objBytes, err := json.Marshal(rawObj)
		require.NoError(t, err, "failed to marshal object")

		switch {
		case apiVersion == "gateway.networking.k8s.io/v1" && kind == "Gateway":
			var gw gwapiv1.Gateway
			err := json.Unmarshal(objBytes, &gw)
			require.NoError(t, err, "failed to unmarshal Gateway")
			res.Gateways[nn] = gw
		case apiVersion == "gateway.networking.k8s.io/v1" && kind == "GatewayClass":
			var gc gwapiv1.GatewayClass
			err := json.Unmarshal(objBytes, &gc)
			require.NoError(t, err, "failed to unmarshal GatewayClass")
			res.GatewayClasses[nn] = gc
		case apiVersion == "gateway.networking.k8s.io/v1" && kind == "HTTPRoute":
			var hr gwapiv1.HTTPRoute
			err := json.Unmarshal(objBytes, &hr)
			require.NoError(t, err, "failed to unmarshal HTTPRoute")
			res.HTTPRoutes[nn] = hr
		case apiVersion == "gateway.networking.k8s.io/v1" && kind == "GRPCRoute":
			var gr gwapiv1.GRPCRoute
			err := json.Unmarshal(objBytes, &gr)
			require.NoError(t, err, "failed to unmarshal GRPCRoute")
			res.GRPCRoutes[nn] = gr
		case apiVersion == "gateway.networking.k8s.io/v1alpha2" && kind == "TLSRoute":
			var tr v1alpha2.TLSRoute
			err := json.Unmarshal(objBytes, &tr)
			require.NoError(t, err, "failed to unmarshal TLSRoute")
			res.TLSRoutes[nn] = tr
		case apiVersion == "gateway.networking.k8s.io/v1alpha2" && kind == "TCPRoute":
			var tcpr v1alpha2.TCPRoute
			err := json.Unmarshal(objBytes, &tcpr)
			require.NoError(t, err, "failed to unmarshal TCPRoute")
			res.TCPRoutes[nn] = tcpr
		case apiVersion == "gateway.networking.k8s.io/v1alpha2" && kind == "UDPRoute":
			var udpr v1alpha2.UDPRoute
			err := json.Unmarshal(objBytes, &udpr)
			require.NoError(t, err, "failed to unmarshal UDPRoute")
			res.UDPRoutes[nn] = udpr
		case apiVersion == "gateway.networking.k8s.io/v1" && kind == "BackendTLSPolicy":
			var btls gwapiv1.BackendTLSPolicy
			err := json.Unmarshal(objBytes, &btls)
			require.NoError(t, err, "failed to unmarshal BackendTLSPolicy")
			res.BackendTLSPolicies[nn] = btls
		case apiVersion == "gateway.networking.k8s.io/v1beta1" && kind == "ReferenceGrant":
			var rg v1beta1.ReferenceGrant
			err := json.Unmarshal(objBytes, &rg)
			require.NoError(t, err, "failed to unmarshal ReferenceGrant")
			res.ReferenceGrants[nn] = rg
		}
	}

	return []i2gw.GatewayResources{res}
}

type ingressBuilder struct {
	*networkingv1.Ingress
}

func (b *ingressBuilder) withName(name string) *ingressBuilder {
	b.ObjectMeta.Name = name

	return b
}

func (b *ingressBuilder) withIngressClass(className string) *ingressBuilder {
	b.Spec.IngressClassName = ptr.To(className)

	return b
}

// Sets the Host field of the first rule in the ingress to the specified string. Does nothing if
// there are no rules.
func (b *ingressBuilder) withHost(host string) *ingressBuilder {
	if len(b.Spec.Rules) > 0 {
		b.Spec.Rules[0].Host = host
	}

	return b
}

func (b *ingressBuilder) withPath(path string) *ingressBuilder {
	for i := range b.Spec.Rules {
		rule := b.Spec.Rules[i]
		for j := range b.Spec.Rules[i].IngressRuleValue.HTTP.Paths {
			p := rule.IngressRuleValue.HTTP.Paths[j]
			p.Path = path
			rule.IngressRuleValue.HTTP.Paths[j] = p
		}
		b.Spec.Rules[i] = rule
	}
	return b
}

func (b *ingressBuilder) withBackend(svc string) *ingressBuilder {
	for i := range b.Spec.Rules {
		rule := b.Spec.Rules[i]
		for j := range b.Spec.Rules[i].IngressRuleValue.HTTP.Paths {
			path := rule.IngressRuleValue.HTTP.Paths[j]
			path.Backend.Service.Name = svc
			rule.IngressRuleValue.HTTP.Paths[j] = path
		}
		b.Spec.Rules[i] = rule
	}
	return b
}

func (b *ingressBuilder) withAnnotation(key, value string) *ingressBuilder {
	if b.ObjectMeta.Annotations == nil {
		b.ObjectMeta.Annotations = make(map[string]string)
	}
	b.ObjectMeta.Annotations[key] = value
	return b
}

func (b *ingressBuilder) withTLSSecret(secretName string, hosts ...string) *ingressBuilder {
	b.Spec.TLS = append(b.Spec.TLS, networkingv1.IngressTLS{
		SecretName: secretName,
		Hosts:      hosts,
	})
	return b
}

func (b *ingressBuilder) build() *networkingv1.Ingress {
	return b.Ingress
}

func basicIngress() *ingressBuilder {
	return &ingressBuilder{
		Ingress: &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: ptr.To(networkingv1.PathTypePrefix),
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: DummyAppName1,
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
