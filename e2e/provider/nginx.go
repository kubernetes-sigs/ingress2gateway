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
	nginxIngressChartVersion = "2.8.1" // NGINX Ingress Controller 5.7.1
	nginxIngressChart        = "oci://ghcr.io/nginx/charts/nginx-ingress"
	nginxIngressReleaseName  = "nginx-ingress"
)

// DeployNginxIngress deploys NGINX Ingress Controller via Helm and returns a cleanup function.
func DeployNginxIngress(
	ctx context.Context,
	l framework.Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying NGINX Ingress Controller (chart %s)", nginxIngressChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	values := map[string]interface{}{
		"controller": map[string]interface{}{
			"ingressClass": map[string]interface{}{
				"name": framework.NginxIngressClass,
			},
			// The tests only exercise Ingress resources. Disabling custom resources avoids
			// installing the VirtualServer/Policy/etc. CRDs.
			"enableCustomResources": false,
			"telemetryReporting": map[string]interface{}{
				"enable": false,
			},
			"service": map[string]interface{}{
				// Traffic is sent through a port forward, so a LoadBalancer isn't needed.
				"type": "ClusterIP",
			},
		},
	}

	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		"",
		nginxIngressReleaseName,
		nginxIngressChart,
		nginxIngressChartVersion,
		namespace,
		true,
		true, // skip CRDs: custom resources are disabled
		values,
	); err != nil {
		return nil, fmt.Errorf("installing chart: %w", err)
	}

	l.Logf("Waiting for NGINX Ingress Controller to be ready")
	if err := framework.WaitForServiceReady(ctx, client, namespace, nginxIngressReleaseName+"-controller"); err != nil {
		return nil, fmt.Errorf("waiting for NGINX Ingress Controller service: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of NGINX Ingress Controller")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up NGINX Ingress Controller")
		if err := framework.UninstallChart(cleanupCtx, settings, nginxIngressReleaseName, namespace); err != nil {
			log.Printf("Uninstalling NGINX Ingress Controller chart: %v", err)
		}

		if err := framework.DeleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting NGINX Ingress Controller namespace: %v", err)
		}
	}, nil
}
