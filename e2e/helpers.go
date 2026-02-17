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
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/provider"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	"k8s.io/client-go/kubernetes"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// NOTE: Setting the Host field in ingress rules and in verifiers is optional. When omitted, a
// random host is generated and used automatically for all ingress objects and verifiers in the
// test case. Most test cases likely don't need an explicit Host value since the value doesn't
// matter as long as the verifier verifies the correct Host. In case a specific Host value is
// important for some test cases, it's important to pay attention to duplicate Host values across
// test cases: While k8s allows defining multiple ingress objects with identical Host values,
// whether doing so makes sense (or even works) depends on the ingress controller and can influence
// test results.

// Wraps framework.RunTestCase with the concrete deploy functions defined in this package, keeping
// provider-specific deployment code out of the framework.
func runTestCase(t *testing.T, tc *framework.TestCase) {
	framework.RunTestCase(t, tc, deployProviders, deployGatewayImplementation)
}

func deployProviders(
	ctx context.Context,
	t *testing.T,
	k8sClient *kubernetes.Clientset,
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
			// If Kong is both the provider and the gateway implementation, skip deploying the Kong
			// provider separately. The gateway deployment handles both ingress and Gateway API.
			if gwImpl == kong.Name {
				continue
			}
			ns := fmt.Sprintf("%s-kong", framework.E2EPrefix)
			r = framework.GlobalResourceManager.Acquire(kong.Name, func() (framework.CleanupFunc, error) {
				return provider.DeployKongIngress(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
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
) framework.Resource {
	var r framework.Resource

	switch gwImpl {
	case istio.ProviderName:
		ns := fmt.Sprintf("%s-istio-system", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(istio.ProviderName, func() (framework.CleanupFunc, error) {
			return implementation.DeployGatewayAPIIstio(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
		})
	case kong.Name:
		ns := fmt.Sprintf("%s-kong", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(kong.Name, func() (framework.CleanupFunc, error) {
			return implementation.DeployGatewayAPIKong(ctx, t, k8sClient, gwClient, kubeconfig, ns, skipCleanup)
		})
	case implementation.KgatewayName:
		ns := fmt.Sprintf("%s-kgateway-system", framework.E2EPrefix)
		r = framework.GlobalResourceManager.Acquire(implementation.KgatewayName, func() (framework.CleanupFunc, error) {
			return implementation.DeployGatewayAPIKgateway(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
		})
	default:
		t.Fatalf("Unknown gateway implementation: %s", gwImpl)
	}

	return r
}
