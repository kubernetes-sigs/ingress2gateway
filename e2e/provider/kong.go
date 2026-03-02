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

package provider

import (
	"context"

	"github.com/kubernetes-sigs/ingress2gateway/e2e/framework"
	"github.com/kubernetes-sigs/ingress2gateway/e2e/implementation"
	"k8s.io/client-go/kubernetes"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// DeployKong deploys Kong via Helm. Kong is unique in that it serves as both an ingress provider
// and a Gateway API implementation. The canonical deploy logic lives in the implementation
// package: this wrapper re-exports it so callers can use provider.DeployKong for consistency with
// other providers.
func DeployKong(
	ctx context.Context,
	l framework.Logger,
	client *kubernetes.Clientset,
	gwClient *gwclientset.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	return implementation.DeployKong(ctx, l, client, gwClient, kubeconfigPath, namespace, skipCleanup)
}
