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
	agentgatewayName        = "agentgateway"
	agentgatewayVersion     = "v1.0.0-rc.1"
	agentgatewayChart       = "oci://cr.agentgateway.dev/charts/agentgateway"
	agentgatewayCRDsChart   = "oci://cr.agentgateway.dev/charts/agentgateway-crds"
	agentgatewayReleaseName = "agentgateway"
	agentgatewayCRDsRelease = "agentgateway-crds"
)

func DeployAgentgateway(
	ctx context.Context,
	l framework.Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying agentgateway %s", agentgatewayVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	// Install CRDs first to avoid races creating extension resources.
	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		"",
		agentgatewayCRDsRelease,
		agentgatewayCRDsChart,
		agentgatewayVersion,
		namespace,
		true,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing agentgateway CRDs chart: %w", err)
	}

	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		"",
		agentgatewayReleaseName,
		agentgatewayChart,
		agentgatewayVersion,
		namespace,
		false,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing agentgateway chart: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of agentgateway")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up agentgateway")
		if err := framework.UninstallChart(cleanupCtx, settings, agentgatewayReleaseName, namespace); err != nil {
			log.Printf("Uninstalling agentgateway chart: %v", err)
		}
		if err := framework.UninstallChart(cleanupCtx, settings, agentgatewayCRDsRelease, namespace); err != nil {
			log.Printf("Uninstalling agentgateway CRDs chart: %v", err)
		}

		if err := framework.DeleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
