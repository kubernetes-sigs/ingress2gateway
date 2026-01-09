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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func createNamespace(ctx context.Context, l logger, client *kubernetes.Clientset, ns string, skipCleanup bool) (func(), error) {
	// Check if namespace already exists. This should be very rare since we use a random suffix,
	// but we check just in case to avoid flaky tests due to conflicts.
	_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		return nil, fmt.Errorf("namespace %s already exists", ns)
	}

	l.Logf("Creating namespace %s", ns)
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating namespace %s: %w", ns, err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of namespace %s", ns)
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("Cleaning up namespace %s", ns)
		if err := deleteNamespaceAndWait(cleanupCtx, client, ns); err != nil {
			log.Printf("Deleting namespace %s: %v", ns, err)
		}
	}, nil
}

func deleteNamespaceAndWait(ctx context.Context, client *kubernetes.Clientset, ns string) error {
	if err := client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting namespace %s: %w", ns, err)
	}

	if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		return fmt.Errorf("waiting for namespace %s to delete: %w", ns, err)
	}

	return nil
}
