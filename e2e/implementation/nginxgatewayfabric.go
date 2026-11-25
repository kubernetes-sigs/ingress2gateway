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

package implementation

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"helm.sh/helm/v4/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

const (
	// NginxGatewayFabricName is the name used to identify the NGINX Gateway Fabric implementation.
	// It's also used as the GatewayClass name.
	NginxGatewayFabricName = "nginx-gateway-fabric"
	// NGF 2.6 is the newest release supporting both Gateway API v1.5 (installed by the framework)
	// and Kubernetes 1.31 (the default node image of the kind version used in CI).
	nginxGatewayFabricVersion     = "2.6.8"
	nginxGatewayFabricChart       = "oci://ghcr.io/nginx/charts/nginx-gateway-fabric"
	nginxGatewayFabricReleaseName = "ngf"
)

// DeployNginxGatewayFabric deploys NGINX Gateway Fabric as a Gateway API implementation via Helm
// and returns a cleanup function.
func DeployNginxGatewayFabric(
	ctx context.Context,
	l framework.Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying NGINX Gateway Fabric %s", nginxGatewayFabricVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	values := map[string]interface{}{
		"nginxGateway": map[string]interface{}{
			"gatewayClassName": NginxGatewayFabricName,
			"productTelemetry": map[string]interface{}{
				"enable": false,
			},
		},
		"nginx": map[string]interface{}{
			"service": map[string]interface{}{
				// Traffic is sent through a port forward, so a LoadBalancer isn't needed.
				"type": "ClusterIP",
			},
		},
	}

	// The chart installs NGF's own CRDs (e.g. ClientSettingsPolicy) and creates the GatewayClass.
	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		"",
		nginxGatewayFabricReleaseName,
		nginxGatewayFabricChart,
		nginxGatewayFabricVersion,
		namespace,
		true,
		false,
		values,
	); err != nil {
		return nil, fmt.Errorf("installing chart: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of NGINX Gateway Fabric")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up NGINX Gateway Fabric")
		if err := framework.UninstallChart(cleanupCtx, settings, nginxGatewayFabricReleaseName, namespace); err != nil {
			log.Printf("Uninstalling NGINX Gateway Fabric chart: %v", err)
		}

		if err := framework.DeleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
