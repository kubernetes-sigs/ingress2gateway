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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

const (
	kongChartVersion   = "2.42.0"
	kongChartRepo      = "https://charts.konghq.com"
	kongGatewayClass   = "kong"
	kongControllerName = "konghq.com/kic-gateway-controller"
)

func deployGatewayAPIKong(
	ctx context.Context,
	l logger,
	client *kubernetes.Clientset,
	gwClient *gwclientset.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying Kong Gateway %s", kongChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	// Configure Kong with Gateway API support.
	// publish_service tells the controller which service to use for Gateway status addresses.
	values := map[string]interface{}{
		"ingressController": map[string]interface{}{
			"env": map[string]interface{}{
				"publish_service": fmt.Sprintf("%s/kong-kong-proxy", namespace),
			},
		},
	}

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
		values,
	); err != nil {
		return nil, fmt.Errorf("installing Kong chart: %w", err)
	}

	// Create the GatewayClass for Kong.
	// The "konghq.com/gatewayclass-unmanaged" annotation is required for Kong to reconcile
	// Gateway resources in unmanaged mode (where Kong doesn't automatically provision Deployments).
	gc := &gwapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: kongGatewayClass,
			Annotations: map[string]string{
				"konghq.com/gatewayclass-unmanaged": "true",
			},
		},
		Spec: gwapiv1.GatewayClassSpec{
			ControllerName: gwapiv1.GatewayController(kongControllerName),
		},
	}

	l.Logf("Creating GatewayClass %s", kongGatewayClass)
	_, err := gwClient.GatewayV1().GatewayClasses().Create(ctx, gc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating GatewayClass: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of Kong Gateway")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up Kong Gateway")

		// Delete the GatewayClass.
		log.Printf("Deleting GatewayClass %s", kongGatewayClass)
		if err := gwClient.GatewayV1().GatewayClasses().Delete(cleanupCtx, kongGatewayClass, metav1.DeleteOptions{}); err != nil {
			log.Printf("Deleting GatewayClass: %v", err)
		}

		if err := uninstallChart(cleanupCtx, settings, "kong", namespace); err != nil {
			log.Printf("Uninstalling Kong chart: %v", err)
		}

		if err := deleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
