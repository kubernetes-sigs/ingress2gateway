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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	ingressNginxChartVersion = "4.14.1"
	ingressNginxChartRepo    = "https://kubernetes.github.io/ingress-nginx"
)

// DeployIngressNginx deploys the ingress-nginx ingress controller via Helm and returns a cleanup
// function.
func DeployIngressNginx(
	ctx context.Context,
	l framework.Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying ingress-nginx %s", ingressNginxChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	// Configure the admission webhook to only validate ingresses with the nginx ingress class.
	// Without this, the webhook intercepts ALL ingresses cluster-wide, which causes failures when
	// other ingress controllers try to create ingresses before the nginx webhook service is ready.
	values := map[string]interface{}{
		"controller": map[string]interface{}{
			"admissionWebhooks": map[string]interface{}{
				"objectSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app.kubernetes.io/ingress-class": "nginx",
					},
				},
			},
		},
	}

	if err := framework.InstallChart(
		ctx,
		l,
		settings,
		ingressNginxChartRepo,
		"ingress-nginx",
		"ingress-nginx",
		ingressNginxChartVersion,
		namespace,
		true,
		values,
	); err != nil {
		return nil, fmt.Errorf("installing chart: %w", err)
	}

	// Wait for the admission webhook service to be ready. The ValidatingWebhookConfiguration is
	// registered cluster-wide immediately, but the admission controller pod takes time to start.
	// Any Ingress creation will fail until the webhook service is ready to handle requests.
	l.Logf("Waiting for ingress-nginx admission webhook to be ready")
	if err := framework.WaitForServiceReady(ctx, client, namespace, "ingress-nginx-controller-admission"); err != nil {
		return nil, fmt.Errorf("waiting for admission webhook service: %w", err)
	}

	// Wait for the CA bundle to be propagated to the ValidatingWebhookConfiguration. The service
	// being ready doesn't guarantee this, which can cause X.509 certificate errors.
	l.Logf("Verifying ingress-nginx admission webhook has CA bundle")
	if err := waitForAdmissionWebhookReady(ctx, client); err != nil {
		return nil, fmt.Errorf("waiting for admission webhook CA bundle: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of ingress-nginx")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up ingress-nginx")
		if err := framework.UninstallChart(cleanupCtx, settings, "ingress-nginx", namespace); err != nil {
			log.Printf("Uninstalling chart: %v", err)
		}

		if err := framework.DeleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}

// Waits until the ingress-nginx ValidatingWebhookConfiguration has a CA bundle configured. The
// webhook service being ready doesn't guarantee the CA bundle has been propagated, which causes
// X.509 certificate verification errors.
func waitForAdmissionWebhookReady(ctx context.Context, client *kubernetes.Clientset) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		vwc, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
			ctx, "ingress-nginx-admission", metav1.GetOptions{})
		if err != nil {
			//nolint:nilerr // Wait function - we deliberately return a nil error here
			return false, nil
		}

		for _, wh := range vwc.Webhooks {
			if len(wh.ClientConfig.CABundle) == 0 {
				return false, nil
			}
		}

		return true, nil
	})
}
