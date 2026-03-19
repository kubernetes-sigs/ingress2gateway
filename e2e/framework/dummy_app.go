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

package framework

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
	image   = "registry.k8s.io/e2e-test-images/agnhost"
	version = "2.39"
)

// DeployDummyApp deploys a dummy backend application. If serverSecretName is
// non-empty the app is deployed with TLS, mounting the named secret.
func DeployDummyApp(ctx context.Context, l Logger, client *kubernetes.Clientset, name, namespace string, skipCleanup bool, serverSecretName string) (func(), error) {
	return deployDummyApp(ctx, l, client, name, namespace, skipCleanup, serverSecretName)
}

// Creates a dummy backend application for testing and returns a cleanup function.
func deployDummyApp(ctx context.Context, l Logger, client *kubernetes.Clientset, name, namespace string, skipCleanup bool, serverSecretName string) (func(), error) {
	if err := createDummyAppDeployment(ctx, l, client, name, namespace, serverSecretName); err != nil {
		return nil, fmt.Errorf("creating deployment: %w", err)
	}

	if err := createDummyAppService(ctx, client, name, namespace, serverSecretName != ""); err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}

	if err := waitForDummyApp(ctx, l, client, name, namespace); err != nil {
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

func createDummyAppDeployment(ctx context.Context, l Logger, client *kubernetes.Clientset, name, namespace, serverSecretName string) error {
	labels := map[string]string{"app": name}

	l.Logf("Creating dummy app %s", name)

	containerArgs := []string{"netexec", "--http-port=8080"}
	portName := "http"
	if serverSecretName != "" {
		containerArgs = append(containerArgs,
			"--tls-cert-file=/etc/tls/tls.crt",
			"--tls-private-key-file=/etc/tls/tls.key",
		)
		portName = "https"
	}

	volumeMount := corev1.VolumeMount{
		Name:      "tls-certs",
		MountPath: "/etc/tls",
		ReadOnly:  true,
	}

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
							Args:  containerArgs,
							Ports: []corev1.ContainerPort{
								{
									Name:          portName,
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	if serverSecretName != "" {
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{volumeMount}
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: serverSecretName,
					},
				},
			},
		}
	}

	if _, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating deployment: %w", err)
	}

	return nil
}

func createDummyAppService(ctx context.Context, client *kubernetes.Clientset, name, namespace string, useTLS bool) error {
	servicePortName := "http"
	var servicePort int32 = 80
	if useTLS {
		servicePortName = "https"
		servicePort = 443
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports: []corev1.ServicePort{
				{
					Name:       servicePortName,
					Port:       servicePort,
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

func waitForDummyApp(ctx context.Context, l Logger, client *kubernetes.Clientset, name, namespace string) error {
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
