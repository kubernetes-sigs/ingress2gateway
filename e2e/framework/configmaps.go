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

package framework

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createConfigMaps(ctx context.Context, l Logger, client *kubernetes.Clientset, ns string, configMaps []*corev1.ConfigMap, skipCleanup bool) (func(), error) {
	for _, cm := range configMaps {
		if cm.Namespace == "" {
			cm.Namespace = ns
		}

		y, err := toYAML(cm)
		if err != nil {
			return nil, fmt.Errorf("converting configmap to YAML: %w", err)
		}

		l.Logf("Creating configmap:\n%s", y)

		_, err = client.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating configmap %s/%s: %w", cm.Namespace, cm.Name, err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of configmaps")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, cm := range configMaps {
			namespace := cm.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting configmap %s/%s", namespace, cm.Name)
			err := client.CoreV1().ConfigMaps(namespace).Delete(cleanupCtx, cm.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting configmap %s: %v", cm.Name, err)
			}
		}
	}, nil
}
