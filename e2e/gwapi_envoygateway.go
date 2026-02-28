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

	"helm.sh/helm/v4/pkg/cli"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

const (
	envoyGatewayName          = "envoy-gateway"
	envoyGatewayVersion       = "v0.0.0-latest"
	envoyGatewayChart         = "oci://docker.io/envoyproxy/gateway-helm"
	envoyGatewayReleaseName   = "envoy-gateway"
	envoyGatewayContrllerName = "gateway.envoyproxy.io/gatewayclass-controller"
)

func deployGatewayAPIEnvoyGateway(
	ctx context.Context,
	l logger,
	client *kubernetes.Clientset,
	gwClient *gwclientset.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying Envoy Gateway %s", envoyGatewayVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	values := map[string]interface{}{
		"config": map[string]interface{}{
			"envoyGateway": map[string]interface{}{
				"provider": map[string]interface{}{
					"type": "Kubernetes",
					"kubernetes": map[string]interface{}{
						"deploy": map[string]interface{}{
							"type": "GatewayNamespace",
						},
					},
				},
			},
		},
	}

	if err := installChart(
		ctx,
		l,
		settings,
		"",
		envoyGatewayReleaseName,
		envoyGatewayChart,
		envoyGatewayVersion,
		namespace,
		true,
		values,
	); err != nil {
		return nil, fmt.Errorf("installing Envoy Gateway chart: %w", err)
	}

	// Create the GatewayClass for Envoy Gateway.
	gc := &gwapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: envoyGatewayName,
		},
		Spec: gwapiv1.GatewayClassSpec{
			ControllerName: gwapiv1.GatewayController(envoyGatewayContrllerName),
		},
	}

	l.Logf("Creating GatewayClass %s", envoyGatewayName)
	_, err := gwClient.GatewayV1().GatewayClasses().Create(ctx, gc, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		_, err = gwClient.GatewayV1().GatewayClasses().Update(ctx, gc, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("creating GatewayClass: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of Envoy Gateway")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up Envoy Gateway")

		// Delete the GatewayClass.
		log.Printf("Deleting GatewayClass %s", envoyGatewayName)
		if err := gwClient.GatewayV1().GatewayClasses().Delete(cleanupCtx, envoyGatewayName, metav1.DeleteOptions{}); err != nil {
			log.Printf("Deleting GatewayClass: %v", err)
		}

		if err := uninstallChart(cleanupCtx, settings, envoyGatewayReleaseName, namespace); err != nil {
			log.Printf("Uninstalling Envoy Gateway chart: %v", err)
		}

		if err := deleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
