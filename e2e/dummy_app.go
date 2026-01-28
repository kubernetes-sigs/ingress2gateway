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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	name    = "dummy-app"
	image   = "registry.k8s.io/e2e-test-images/agnhost"
	version = "2.39"
)

func deployDummyApp(ctx context.Context, l logger, client *kubernetes.Clientset, namespace string, skipCleanup bool) (func(), error) {
	if err := createDummyAppDeployment(ctx, l, client, namespace); err != nil {
		return nil, fmt.Errorf("creating deployment: %w", err)
	}

	if err := createDummyAppService(ctx, client, namespace); err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}

	if err := waitForDummyApp(ctx, l, client, namespace); err != nil {
		return nil, fmt.Errorf("waiting for dummy app: %w", err)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			log.Printf("Skipping cleanup of dummy app")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		log.Printf("Deleting dummy app %s", name)
		err := client.CoreV1().Services(namespace).Delete(cleanupCtx, name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Deleting service %s: %v", name, err)
		}

		err = client.AppsV1().Deployments(namespace).Delete(cleanupCtx, name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Deleting deployment %s: %v", name, err)
		}
	}, nil
}

func createDummyAppDeployment(ctx context.Context, l logger, client *kubernetes.Clientset, namespace string) error {
	labels := map[string]string{"app": name}

	l.Logf("Creating dummy app %s", name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: fmt.Sprintf("%s:%s", image, version),
							Args:  []string{"netexec", "--http-port=8080"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	if _, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating deployment: %w", err)
	}

	return nil
}

func createDummyAppService(ctx context.Context, client *kubernetes.Clientset, namespace string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	if _, err := client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return nil
}

func waitForDummyApp(ctx context.Context, l logger, client *kubernetes.Clientset, namespace string) error {
	l.Logf("Waiting for dummy app to be ready")
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		dep, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			//nolint:nilerr // Wait function - we deliberately return a nil error here
			return false, nil
		}
		for _, cond := range dep.Status.Conditions {
			if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for deployment: %w", err)
	}

	return nil
}
