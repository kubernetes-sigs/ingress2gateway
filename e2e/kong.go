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

func deployKongIngress(
	ctx context.Context,
	l logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying Kong Ingress Controller %s", kongChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	if err := installChart(
		ctx,
		l,
		settings,
		kongChartRepo,
		"kong",
		"kong",
		kongChartVersion,
		namespace,
		true,
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing Kong Ingress chart: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of Kong Ingress Controller")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up Kong Ingress Controller")
		if err := uninstallChart(cleanupCtx, settings, "kong", namespace); err != nil {
			log.Printf("Uninstalling Kong Ingress chart: %v", err)
		}

		if err := deleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
