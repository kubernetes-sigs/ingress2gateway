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
	"net/http"
	"net/url"
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	"golang.org/x/sync/semaphore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Limits the number of concurrent port forward connections to avoid exhausting API server
// resources when running tests in parallel.
const maxConcurrentPortForwards = 50

// A package-level semaphore to limit concurrent port forwards.
var portForwardSem = semaphore.NewWeighted(maxConcurrentPortForwards)

// Manages a port forward connection to a Kubernetes pod using client-go.
type portForwarder struct {
	stopChan   chan struct{}
	pf         *portforward.PortForwarder
	localPort  uint16
	releaseSem func()
}

// Creates a port forward to a service's backing pod using client-go. The caller must call Stop()
// when done to release resources.
func startPortForwardToService(
	ctx context.Context,
	client *kubernetes.Clientset,
	restConfig *rest.Config,
	namespace string,
	serviceName string,
	servicePort int,
) (*portForwarder, string, error) {
	if err := portForwardSem.Acquire(ctx, 1); err != nil {
		return nil, "", fmt.Errorf("acquiring port forward semaphore: %w", err)
	}

	pf, addr, err := startPortForward(ctx, client, restConfig, namespace, serviceName, servicePort)
	if err != nil {
		portForwardSem.Release(1)
		return nil, "", err
	}

	pf.releaseSem = func() { portForwardSem.Release(1) }
	return pf, addr, nil
}

func startPortForward(
	ctx context.Context,
	client *kubernetes.Clientset,
	restConfig *rest.Config,
	namespace string,
	serviceName string,
	servicePort int,
) (*portForwarder, string, error) {
	svc, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("getting service %s/%s: %w", namespace, serviceName, err)
	}

	if len(svc.Spec.Selector) == 0 {
		return nil, "", fmt.Errorf("service %s/%s has no selector", namespace, serviceName)
	}

	// List pods for service.
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: svc.Spec.Selector}),
	})
	if err != nil {
		return nil, "", fmt.Errorf("listing pods for service %s/%s: %w", namespace, serviceName, err)
	}

	if len(pods.Items) == 0 {
		return nil, "", fmt.Errorf("no pods found for service %s/%s", namespace, serviceName)
	}

	// client-go doesn't support port forwarding to services, only to pods. Look for a pod we can
	// forward to.
	pod := findReadyPod(&pods.Items)
	if pod == nil {
		return nil, "", fmt.Errorf("no ready pods found for service %s/%s", namespace, serviceName)
	}

	port, err := findTargetPort(svc, pod, servicePort)
	if err != nil {
		return nil, "", fmt.Errorf("finding target port: %w", err)
	}

	return startPortForwardToPod(ctx, restConfig, namespace, pod.Name, port)
}

func startPortForwardToPod(
	ctx context.Context,
	restConfig *rest.Config,
	namespace string,
	podName string,
	podPort int,
) (*portForwarder, string, error) {
	reqURL, err := url.Parse(fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward",
		restConfig.Host, namespace, podName))
	if err != nil {
		return nil, "", fmt.Errorf("parsing URL: %w", err)
	}

	// Create SPDY transport.
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, "", fmt.Errorf("creating SPDY transport: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)

	// Use "0" as the local port to let the system assign an available port.
	// This avoids race conditions when multiple tests request ports concurrently.
	ports := []string{fmt.Sprintf("0:%d", podPort)}

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, nil, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating port forwarder: %w", err)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case <-readyChan:
		// Port forward is ready.
	case err := <-errChan:
		return nil, "", fmt.Errorf("port forward failed: %w", err)
	case <-ctx.Done():
		close(stopChan)
		return nil, "", ctx.Err()
	}

	// Get the local port assigned by the system (we requested port 0 above).
	forwardedPorts, err := pf.GetPorts()
	if err != nil {
		close(stopChan)
		return nil, "", fmt.Errorf("getting forwarded ports: %w", err)
	}

	if len(forwardedPorts) == 0 {
		close(stopChan)
		return nil, "", fmt.Errorf("no ports forwarded")
	}

	localPort := forwardedPorts[0].Local
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)

	return &portForwarder{
		stopChan:  stopChan,
		pf:        pf,
		localPort: localPort,
	}, addr, nil
}

// Terminates the port forward connection.
func (pf *portForwarder) stop() {
	if pf.stopChan != nil {
		close(pf.stopChan)
		pf.stopChan = nil
	}

	if pf.releaseSem != nil {
		pf.releaseSem()
		pf.releaseSem = nil
	}
}

// Finds a pod that is running, ready, and not terminating.
func findReadyPod(pods *[]corev1.Pod) *corev1.Pod {
	for i := range *pods {
		pod := &(*pods)[i]
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if pod.DeletionTimestamp != nil {
			continue // Pod is terminating
		}
		// Check Ready condition.
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return pod
			}
		}
	}

	return nil
}

// Resolves the service port to a container port on the pod.
func findTargetPort(svc *corev1.Service, pod *corev1.Pod, servicePort int) (int, error) {
	for _, sp := range svc.Spec.Ports {
		if int(sp.Port) == servicePort {
			// If targetPort is a number, use it directly.
			if sp.TargetPort.IntValue() != 0 {
				return sp.TargetPort.IntValue(), nil
			}
			// If targetPort is a name, find the container port.
			portName := sp.TargetPort.String()
			for _, container := range pod.Spec.Containers {
				for _, cp := range container.Ports {
					if cp.Name == portName {
						return int(cp.ContainerPort), nil
					}
				}
			}
			return 0, fmt.Errorf("named port %q not found in pod", portName)
		}
	}

	return 0, fmt.Errorf("service port %d not found", servicePort)
}

// Finds the service created by a Gateway API implementation for a Gateway object.
func findGatewayService(
	ctx context.Context,
	log logger,
	client *kubernetes.Clientset,
	gatewayNamespace string,
	gatewayName string,
) (*corev1.Service, error) {
	var out *corev1.Service

	// The service may not exist immediately after Gateway creation. Wait for it.
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		// Different implementations use different naming/labeling conventions, so we try multiple
		// strategies.

		// Try the standard Gateway API label (used by Istio and others).
		// See: https://gateway-api.sigs.k8s.io/geps/gep-1762/#resource-attachment
		selector := fmt.Sprintf("gateway.networking.k8s.io/gateway-name=%s", gatewayName)
		services, err := client.CoreV1().Services(gatewayNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
		if err == nil && len(services.Items) > 0 {
			if len(services.Items) > 1 {
				log.Logf("WARNING: Found %d services for gateway %s - selecting the first one",
					len(services.Items), gatewayName)
			}
			out = &services.Items[0]
			return true, nil
		}

		// Try other implementation-specific label selectors.
		selectors := []string{
			fmt.Sprintf("gateway.envoyproxy.io/owning-gateway-name=%s", gatewayName), // Envoy Gateway
			fmt.Sprintf("app.kubernetes.io/instance=%s", gatewayName),                // NGINX Gateway Fabric
			// TODO: Add labels for more implementations.
		}
		for _, s := range selectors {
			services, err := client.CoreV1().Services(gatewayNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: s,
			})
			if err != nil {
				continue
			}
			if len(services.Items) > 0 {
				out = &services.Items[0]
				return true, nil
			}
		}

		// Fallback: Istio names services as <gateway-name>-<gatewayclass-name>.
		// Common gateway class names: istio, istio-waypoint.
		for _, suffix := range []string{"-istio", ""} {
			svcName := gatewayName + suffix
			svc, err := client.CoreV1().Services(gatewayNamespace).Get(ctx, svcName, metav1.GetOptions{})
			if err == nil {
				out = svc
				return true, nil
			}
		}

		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("finding service for gateway %s/%s: %w", gatewayNamespace, gatewayName, err)
	}

	return out, nil
}

// Finds the service for an ingress controller.
func findIngressControllerService(
	ctx context.Context,
	client *kubernetes.Clientset,
	namespace string,
	controllerName string,
) (*corev1.Service, error) {
	// Map controller names to their typical service names.
	serviceNames := map[string][]string{
		ingressnginx.Name: {"ingress-nginx-controller"},
		kong.Name:         {"kong-kong-proxy"},
	}

	names, ok := serviceNames[controllerName]
	if !ok {
		return nil, fmt.Errorf("unknown ingress controller: %s", controllerName)
	}

	var lastErr error
	for _, name := range names {
		svc, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return svc, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("could not find service for ingress controller %s: %w", controllerName, lastErr)
}
