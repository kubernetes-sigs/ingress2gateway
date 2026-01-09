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
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// Accepts a path to a kubeconfig file and returns a k8s client set.
func newClientFromKubeconfigPath(path string) (*kubernetes.Clientset, error) {
	cc, err := configFromKubeconfigPath(path)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(cc)
}

// Accepts a path to a kubeconfig file and returns a rest.Config. This is useful for operations
// that need direct access to the rest config, such as port forwarding.
func newRestConfigFromKubeconfigPath(path string) (*rest.Config, error) {
	return configFromKubeconfigPath(path)
}

// Accepts a path to a kubeconfig file and returns a Gateway API client set.
func newGatewayClientFromKubeconfigPath(path string) (*gwclientset.Clientset, error) {
	cc, err := configFromKubeconfigPath(path)
	if err != nil {
		return nil, err
	}

	return gwclientset.NewForConfig(cc)
}

// Accepts a path to a kubeconfig file and returns an API extensions client set.
func newAPIExtensionsClientFromKubeconfigPath(path string) (*apiextensionsclientset.Clientset, error) {
	cc, err := configFromKubeconfigPath(path)
	if err != nil {
		return nil, err
	}

	return apiextensionsclientset.NewForConfig(cc)
}

// Accepts a path to a kubeconfig file and returns a rest config.
// Configures increased QPS and Burst for parallel test execution.
func configFromKubeconfigPath(path string) (*rest.Config, error) {
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}

	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{},
	)

	restConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	// Increase rate limits for parallel test execution.
	// Default QPS is 5, Burst is 10, which is too low for parallel e2e tests
	// that make many API calls concurrently.
	restConfig.QPS = 50
	restConfig.Burst = 100

	return restConfig, nil
}
