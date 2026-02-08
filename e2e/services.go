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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Waits until at least one of the pods backing the service is ready.
func waitForServiceReady(
	ctx context.Context,
	client *kubernetes.Clientset,
	namespace string,
	serviceName string,
) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		svc, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			// Service doesn't exist yet. Keep waiting.
			//nolint:nilerr // Wait function - we deliberately return a nil error here
			return false, nil
		}

		if len(svc.Spec.Selector) == 0 {
			return false, fmt.Errorf("service %s/%s has no selector", namespace, serviceName)
		}

		selector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: svc.Spec.Selector})
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			//nolint:nilerr // Wait function - we deliberately return a nil error here
			return false, nil
		}

		// Check if at least one pod is ready.
		readyPod := findReadyPod(&pods.Items)
		return readyPod != nil, nil
	})
}
