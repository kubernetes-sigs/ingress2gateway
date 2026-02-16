/*
Copyright 2026 The Kubernetes Authors.

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
	"log"
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"helm.sh/helm/v4/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

const (
	kgatewayName        = "kgateway"
	kgatewayVersion     = "v2.2.0"
	kgatewayChart       = "oci://ghcr.io/kgateway-dev/charts/kgateway"
	kgatewayCRDsChart   = "oci://ghcr.io/kgateway-dev/charts/kgateway-crds"
	kgatewayReleaseName = "kgateway"
	kgatewayCRDsRelease = "kgateway-crds"
)

func deployGatewayAPIKgateway(
	ctx context.Context,
	l framework.Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying kgateway %s", kgatewayVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	// Install CRDs first to avoid races creating extension resources.
	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		"",
		kgatewayCRDsRelease,
		kgatewayCRDsChart,
		kgatewayVersion,
		namespace,
		true,
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing kgateway CRDs chart: %w", err)
	}

	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		"",
		kgatewayReleaseName,
		kgatewayChart,
		kgatewayVersion,
		namespace,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing kgateway chart: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of kgateway")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up kgateway")
		if err := framework.UninstallChart(cleanupCtx, settings, kgatewayReleaseName, namespace); err != nil {
			log.Printf("Uninstalling kgateway chart: %v", err)
		}
		if err := framework.UninstallChart(cleanupCtx, settings, kgatewayCRDsRelease, namespace); err != nil {
			log.Printf("Uninstalling kgateway CRDs chart: %v", err)
		}

		if err := framework.DeleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
