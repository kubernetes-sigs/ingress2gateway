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
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

const (
	envoyGatewayName          = "envoy-gateway"
	envoyGatewayVersion       = "v1.7.0"
	envoyGatewayCRDInstallURL = "https://github.com/envoyproxy/gateway/releases/download/" + envoyGatewayVersion + "/envoy-gateway-crds.yaml"
	envoyGatewayChart         = "oci://docker.io/envoyproxy/gateway-helm"
	envoyGatewayReleaseName   = "envoy-gateway"
	envoyGatewayContrllerName = "gateway.envoyproxy.io/gatewayclass-controller"
)

func deployGatewayAPIEnvoyGateway(
	ctx context.Context,
	l logger,
	client *kubernetes.Clientset,
	apiextClient *apiextensionsclientset.Clientset,
	gwClient *gwclientset.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	l.Logf("Deploying Envoy Gateway %s", envoyGatewayVersion)

	// Install Envoy Gateway CRDs via server-side apply.
	// helm install cannot be used here due to a known Helm limitation where large CRDs
	// in the templates/ directory cause the release Secret to exceed the 1MB size limit.
	// See: https://gateway.envoyproxy.io/v1.7/install/install-helm/#installing-crds-separately
	cleanupCRDs, err := deployCRDs(ctx, l, apiextClient, envoyGatewayCRDInstallURL, skipCleanup)
	if err != nil {
		return nil, fmt.Errorf("installing Envoy Gateway CRDs: %w", err)
	}

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	values := map[string]any{
		"config": map[string]any{
			"envoyGateway": map[string]any{
				"provider": map[string]any{
					"type": "Kubernetes",
					"kubernetes": map[string]any{
						"deploy": map[string]any{
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
		true,
		values,
	); err != nil {
		cleanupCRDs()
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
	_, err = gwClient.GatewayV1().GatewayClasses().Create(ctx, gc, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		_, err = gwClient.GatewayV1().GatewayClasses().Update(ctx, gc, metav1.UpdateOptions{})
	}
	if err != nil {
		cleanupCRDs()
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

		cleanupCRDs()

		if err := deleteNamespaceAndWait(cleanupCtx, client, namespace); err != nil {
			log.Printf("Deleting namespace: %v", err)
		}
	}, nil
}
