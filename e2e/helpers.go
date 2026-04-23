/*
Copyright The Kubernetes Authors.

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
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/provider"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/traefik"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// setupTestEnv wraps framework.SetupTestEnv with the concrete deploy functions defined in this
// package. The returned TestEnv can be used for additional test-specific setup before calling Run().
func setupTestEnv(t *testing.T, providers []string, gatewayImplementation string) *framework.TestEnv {
	return framework.SetupTestEnv(t, providers, gatewayImplementation, deployProviders, deployGatewayImplementation)
}

// runTestCase is a convenience wrapper for tests that don't need custom setup between environment
// creation and test execution.
func runTestCase(t *testing.T, tc *framework.TestCase) {
	setupTestEnv(t, tc.Providers, tc.GatewayImplementation).Run(tc)
}

func deployProviders(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
	gwClient *gwclientset.Clientset,
	kubeconfig string,
	providers []string,
	gwImpl string,
	skipCleanup bool,
) []framework.Resource {
	var resources []framework.Resource

	for _, p := range providers {
		var r framework.Resource
		switch p {
		case ingressnginx.Name:
			ns := fmt.Sprintf("%s-ingress-nginx", framework.E2EPrefix)
			r = framework.GlobalResourceManager.Acquire(ingressnginx.Name, func() (framework.CleanupFunc, error) {
				return provider.DeployIngressNginx(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
			})
		case kong.Name:
			ns := fmt.Sprintf("%s-kong", framework.E2EPrefix)
			// We use implementation.KongName as the acquire key so that Kong is deployed at most
			// once, regardless of whether it appears as an ingress provider, a gateway
			// implementation or both. ResourceManager.Acquire deduplicates by key.
			r = framework.GlobalResourceManager.Acquire(implementation.KongName, func() (framework.CleanupFunc, error) {
				return provider.DeployKong(ctx, t, k8sClient, gwClient, kubeconfig, ns, skipCleanup)
			})
		case traefik.Name:
			ns := fmt.Sprintf("%s-traefik", framework.E2EPrefix)
			r = framework.GlobalResourceManager.Acquire(traefik.Name, func() (framework.CleanupFunc, error) {
				return provider.DeployTraefik(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
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
	apiextClient *apiextensionsclientset.Clientset,
	gwClient *gwclientset.Clientset,
	kubeconfig string,
	gwImpl string,
	skipCleanup bool,
) framework.Resource {
	var r framework.Resource

	switch gwImpl {
	case implementation.IstioName:
		ns := fmt.Sprintf("%s-istio-system", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(implementation.IstioName, func() (framework.CleanupFunc, error) {
			return implementation.DeployIstio(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
		})
	case implementation.KongName:
		ns := fmt.Sprintf("%s-kong", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(implementation.KongName, func() (framework.CleanupFunc, error) {
			return implementation.DeployKong(ctx, t, k8sClient, gwClient, kubeconfig, ns, skipCleanup)
		})
	case implementation.KgatewayName:
		ns := fmt.Sprintf("%s-kgateway-system", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(implementation.KgatewayName, func() (framework.CleanupFunc, error) {
			return implementation.DeployKgateway(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
		})
	case implementation.EnvoyGatewayName:
		ns := fmt.Sprintf("%s-envoy-gateway-system", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(implementation.EnvoyGatewayName, func() (framework.CleanupFunc, error) {
			return implementation.DeployEnvoyGateway(ctx, t, k8sClient, apiextClient, gwClient, kubeconfig, ns, skipCleanup)
		})
	case implementation.AgentgatewayName:
		ns := fmt.Sprintf("%s-agentgateway-system", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(implementation.AgentgatewayName, func() (framework.CleanupFunc, error) {
			return implementation.DeployAgentgateway(ctx, t, k8sClient, gwClient, kubeconfig, ns, skipCleanup)
		})
	default:
		t.Fatalf("Unknown gateway implementation: %s", gwImpl)
	}

	return r
}
