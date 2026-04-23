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

package provider

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
	traefikChartVersion = "34.4.1"
	traefikChartRepo    = "https://helm.traefik.io/traefik"
)

// DeployTraefik deploys the Traefik ingress controller via Helm and returns a cleanup function.
func DeployTraefik(
	ctx context.Context,
	l framework.Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying Traefik %s", traefikChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		traefikChartRepo,
		"traefik",
		"traefik",
		traefikChartVersion,
		namespace,
		true,
		true, // skip CRDs: Gateway API CRDs are already installed by the test framework
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing chart: %w", err)
	}

	l.Logf("Waiting for Traefik to be ready")
	if err := framework.WaitForServiceReady(ctx, client, namespace, "traefik"); err != nil {
		return nil, fmt.Errorf("waiting for Traefik service: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of Traefik")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up Traefik")
		if err := framework.UninstallChart(cleanupCtx, settings, "traefik", namespace); err != nil {
			log.Printf("Uninstalling Traefik chart: %v", err)
		}

		if err := framework.DeleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting Traefik namespace: %v", err)
		}
	}, nil
}
