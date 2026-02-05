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

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createIngresses(ctx context.Context, l logger, client *kubernetes.Clientset, ns string, ingresses []*networkingv1.Ingress, skipCleanup bool) (func(), error) {
	for _, ingress := range ingresses {
		// Add ingress class label for admission webhook selectors. This allows ingress controllers
		// to configure their admission webhooks to only validate ingresses with matching labels,
		// avoiding cross-controller interference in parallel test scenarios.
		if ingress.Spec.IngressClassName != nil {
			if ingress.Labels == nil {
				ingress.Labels = make(map[string]string)
			}
			ingress.Labels["app.kubernetes.io/ingress-class"] = *ingress.Spec.IngressClassName
		}

		y, err := toYAML(ingress)
		if err != nil {
			return nil, fmt.Errorf("converting ingress to YAML: %w", err)
		}

		l.Logf("Creating ingress:\n%s", y)

		_, err = client.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating ingress: %w", err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of ingresses")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, ingress := range ingresses {
			log.Printf("Deleting ingress %s", ingress.Name)
			err := client.NetworkingV1().Ingresses(ns).Delete(cleanupCtx, ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting ingress %s: %v", ingress.Name, err)
			}
		}
	}, nil
}
