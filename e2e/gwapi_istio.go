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
	"log"
	"time"

	"helm.sh/helm/v4/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

const (
	istioVersion   = "1.28.2"
	istioChartRepo = "https://istio-release.storage.googleapis.com/charts"
)

// Installs Istio with Gateway API support using Helm. Returns a cleanup function that uninstalls
// Istio and deletes the namespace.
func deployGatewayAPIIstio(ctx context.Context,
	l logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying Istio %s", istioVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	values := map[string]interface{}{
		"global": map[string]interface{}{
			"istioNamespace": namespace,
		},
	}

	if err := installChart(
		ctx,
		l,
		settings,
		istioChartRepo,
		"istio-base",
		"base",
		istioVersion,
		namespace,
		true,
		values,
	); err != nil {
		return nil, fmt.Errorf("installing chart %s: %w", "istio-base", err)
	}

	values = map[string]interface{}{
		"global": map[string]interface{}{
			"istioNamespace": namespace,
		},
	}

	if err := installChart(
		ctx,
		l,
		settings,
		istioChartRepo,
		"istiod",
		"istiod",
		istioVersion,
		namespace,
		false,
		values,
	); err != nil {
		return nil, fmt.Errorf("installing chart %s: %w", "istiod", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of Istio")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up Istio")
		if err := uninstallChart(cleanupCtx, settings, "istiod", namespace); err != nil {
			log.Printf("Uninstalling chart %s: %v", "istiod", err)
		}
		if err := uninstallChart(cleanupCtx, settings, "istio-base", namespace); err != nil {
			log.Printf("Uninstalling chart %s: %v", "istio-base", err)
		}

		if err := deleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
