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
	"os"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	// Import providers to register them.
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/apisix"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/cilium"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/gce"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/openapi3"

	// Import emitters to register them.
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitters/standard"
)

// A prefix for all namespaces used in the e2e tests.
const e2ePrefix = "i2gw"

type testCase struct {
	ingresses             []*networkingv1.Ingress
	providers             []string
	providerFlags         map[string]map[string]string
	gatewayImplementation string
	verifiers             map[string][]verifier
}

func runTestCase(t *testing.T, tc *testCase) {
	t.Parallel()

	if len(tc.providers) == 0 {
		t.Fatal("At least one provider must be specified")
	}

	if tc.gatewayImplementation == "" {
		t.Fatal("gatewayImplementation must be specified")
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
	randPrefix, err := randString(5)
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

	providers := deployProviders(ctx, t, k8sClient, kubeconfig, tc.providers, tc.gatewayImplementation, skipCleanup)
	gwImpl := deployGatewayImplementation(ctx, t, k8sClient, gwClient, kubeconfig, tc.gatewayImplementation, skipCleanup)

	resources := append(providers, gwImpl)

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

	cleanupDummyApp, err := deployDummyApp(ctx, t, k8sClient, appNS, skipCleanup)
	require.NoError(t, err, "creating dummy app")
	t.Cleanup(cleanupDummyApp)

	// Populate ingress Host field if not specified in the test case.
	for _, ing := range tc.ingresses {
		for i := range ing.Spec.Rules {
			if ing.Spec.Rules[i].Host == "" {
				ing.Spec.Rules[i].Host = fmt.Sprintf("%s.%s.%s.test", ing.Name, randPrefix, e2ePrefix)
			}
		}
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
	)
	t.Cleanup(func() {
		for _, pf := range ingressPortForwarders {
			pf.stop()
		}
	})

	verifyIngresses(ctx, t, tc, ingressAddresses)

	// We pass an empty input file to make i2gw read ingresses from the cluster.
	res, notif, err := i2gw.ToGatewayAPIResources(ctx, appNS, "", tc.providers, "standard", tc.providerFlags)
	require.NoError(t, err)

	if len(notif) > 0 {
		t.Log("Received notifications during conversion:")
		for _, table := range notif {
			t.Log(table)
		}
	}

	// TODO: Hack! Force correct gateway class since i2gw doesn't seem to infer that from the
	// ingress at the moment.
	for _, r := range res {
		for k, v := range r.Gateways {
			v.Spec.GatewayClassName = gwapiv1.ObjectName(tc.gatewayImplementation)
			r.Gateways[k] = v
		}
	}

	cleanupGatewayResources, err := createGatewayResources(ctx, t, gwClient, appNS, res, skipCleanup)
	require.NoError(t, err, "creating gateway resources")
	t.Cleanup(cleanupGatewayResources)

	// Set up port forwarding to each gateway for verification.
	gatewayPortForwarders, gwAddresses := setUpGatewayPortForwarding(ctx, t, k8sClient, restConfig, getGateways(res), appNS, tc.gatewayImplementation)
	t.Cleanup(func() {
		for _, pf := range gatewayPortForwarders {
			pf.stop()
		}
	})

	verifyGatewayResources(ctx, t, tc, gwAddresses)
}

func deployProviders(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
	kubeconfig string,
	providers []string,
	gwImpl string,
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
			if gwImpl == kong.Name {
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
) ([]*portForwarder, map[string]string) {
	var pfs []*portForwarder
	addresses := make(map[string]string)

	for _, p := range providers {
		switch p {
		case ingressnginx.Name:
			ingressNS := fmt.Sprintf("%s-ingress-nginx", e2ePrefix)
			svc, err := findIngressControllerService(ctx, k8sClient, ingressNS, "ingress-nginx")
			require.NoError(t, err, "finding ingress-nginx service")

			t.Logf("Waiting for ingress controller %s service %s/%s to have ready pods", p, svc.Namespace, svc.Name)
			err = waitForServiceReady(ctx, k8sClient, svc.Namespace, svc.Name)
			require.NoError(t, err, "waiting for ingress-nginx service to be ready")

			pf, addr, err := startPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 80)
			require.NoError(t, err, "starting port forward to ingress-nginx")
			pfs = append(pfs, pf)
			addresses[ingressnginx.NginxIngressClass] = addr
			t.Logf("Port forwarding ingress controller %s via %s", p, addr)
		case kong.Name:
			// Kong uses the same namespace for both ingress and gateway when both are enabled.
			ingressNS := fmt.Sprintf("%s-kong", e2ePrefix)
			svc, err := findIngressControllerService(ctx, k8sClient, ingressNS, kong.Name)
			require.NoError(t, err, "finding kong service")

			t.Logf("Waiting for ingress controller %s service %s/%s to have ready pods", p, svc.Namespace, svc.Name)
			err = waitForServiceReady(ctx, k8sClient, svc.Namespace, svc.Name)
			require.NoError(t, err, "waiting for kong service to be ready")

			pf, addr, err := startPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 80)
			require.NoError(t, err, "starting port forward to kong")
			pfs = append(pfs, pf)
			addresses[kong.KongIngressClass] = addr
			t.Logf("Port forwarding ingress controller %s via %s", p, addr)
		default:
			t.Fatalf("Unknown ingress provider: %s", p)
		}
	}

	return pfs, addresses
}

func setUpGatewayPortForwarding(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
	restConfig *rest.Config,
	gateways map[types.NamespacedName]gwapiv1.Gateway,
	appNS string,
	gwImpl string,
) ([]*portForwarder, map[string]string) {
	var pfs []*portForwarder
	addresses := make(map[string]string)

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
		addresses[gwName.Name] = addr
		t.Logf("Port forwarding gateway %s via %s", gwName, addr)
	}

	return pfs, addresses
}

func verifyIngresses(ctx context.Context, t *testing.T, tc *testCase, ingressAddresses map[string]string) {
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

func verifyGatewayResources(ctx context.Context, t *testing.T, tc *testCase, gwAddresses map[string]string) {
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
												Name: "dummy-app",
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
