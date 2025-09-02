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

package nginx

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	nginxv1 "github.com/nginx/kubernetes-ingress/pkg/apis/configuration/v1"
)

// NginxIngressClasses contains NGINX IngressClass names
var NginxIngressClasses = sets.New(
	"nginx",
)

// NGINX CRD GroupVersionKind constants
var (
	VirtualServerGVK       = schema.GroupVersionKind{Group: "k8s.nginx.org", Version: "v1", Kind: "VirtualServer"}
	VirtualServerRouteGVK  = schema.GroupVersionKind{Group: "k8s.nginx.org", Version: "v1", Kind: "VirtualServerRoute"}
	TransportServerGVK     = schema.GroupVersionKind{Group: "k8s.nginx.org", Version: "v1", Kind: "TransportServer"}
	GlobalConfigurationGVK = schema.GroupVersionKind{Group: "k8s.nginx.org", Version: "v1", Kind: "GlobalConfiguration"}
)

type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	return &resourceReader{
		conf: conf,
	}
}

// helper constructors for CRD types
func newVirtualServer() *nginxv1.VirtualServer { return &nginxv1.VirtualServer{} }
func newVirtualServerRoute() *nginxv1.VirtualServerRoute {
	return &nginxv1.VirtualServerRoute{}
}
func newTransportServer() *nginxv1.TransportServer {
	return &nginxv1.TransportServer{}
}
func newGlobalConfiguration() *nginxv1.GlobalConfiguration {
	return &nginxv1.GlobalConfiguration{}
}

// parseNamespacedName parses a string in the format "namespace/name" or just "name"
// Returns namespace and name. If no namespace is specified, uses the default namespace.
func parseNamespacedName(namespacedName, defaultNamespace string) (namespace, name string) {
	if strings.Contains(namespacedName, "/") {
		parts := strings.SplitN(namespacedName, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1]
		}
	}
	// If no namespace specified or invalid format, use default namespace
	return defaultNamespace, namespacedName
}

// readResourcesFromCluster reads nginx resources from the Kubernetes cluster
func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourceStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, NginxIngressClasses)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	// Read VirtualServer CRDs
	virtualServers, err := genericReadFromCluster(
		ctx,
		r.conf.Client,
		r.conf.Namespace,
		VirtualServerGVK,
		newVirtualServer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read VirtualServers: %w", err)
	}
	storage.VirtualServers = virtualServers

	// Read VirtualServerRoute CRDs
	virtualServerRoutes, err := genericReadFromCluster(
		ctx,
		r.conf.Client,
		r.conf.Namespace,
		VirtualServerRouteGVK,
		newVirtualServerRoute,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read VirtualServerRoutes: %w", err)
	}
	storage.VirtualServerRoutes = virtualServerRoutes

	// Read TransportServer CRDs
	transportServers, err := genericReadFromCluster(
		ctx,
		r.conf.Client,
		r.conf.Namespace,
		TransportServerGVK,
		newTransportServer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read TransportServers: %w", err)
	}
	storage.TransportServers = transportServers

	// Read single GlobalConfiguration specified by flag
	if flags, ok := r.conf.ProviderSpecificFlags[Name]; ok {
		if gcName := flags[GlobalConfigurationFlag]; gcName != "" {
			var gc nginxv1.GlobalConfiguration
			// Parse namespace/name format, fallback to provider config namespace
			namespace, name := parseNamespacedName(gcName, r.conf.Namespace)
			key := types.NamespacedName{Namespace: namespace, Name: name}
			if err := r.conf.Client.Get(ctx, key, &gc); err != nil {
				return nil, fmt.Errorf("failed to get GlobalConfiguration %s/%s: %w", key.Namespace, key.Name, err)
			}
			storage.GlobalConfiguration = &gc
		}
	}

	return storage, nil
}

// readResourcesFromFile reads nginx resources from a YAML file
func (r *resourceReader) readResourcesFromFile(filename string) (*storage, error) {
	storage := newResourceStorage()

	ingresses, err := common.ReadIngressesFromFile(filename, r.conf.Namespace, NginxIngressClasses)
	if err != nil {
		return nil, err
	}
	storage.Ingresses = ingresses

	// Read VirtualServer CRDs
	virtualServers, err := genericReadFromFile(
		filename,
		r.conf.Namespace,
		VirtualServerGVK,
		newVirtualServer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read VirtualServers: %w", err)
	}
	storage.VirtualServers = virtualServers

	// Read VirtualServerRoute CRDs
	virtualServerRoutes, err := genericReadFromFile(
		filename,
		r.conf.Namespace,
		VirtualServerRouteGVK,
		newVirtualServerRoute,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read VirtualServerRoutes: %w", err)
	}
	storage.VirtualServerRoutes = virtualServerRoutes

	// Read TransportServer CRDs
	transportServers, err := genericReadFromFile(
		filename,
		r.conf.Namespace,
		TransportServerGVK,
		newTransportServer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read TransportServers: %w", err)
	}
	storage.TransportServers = transportServers

	// Read single GlobalConfiguration from file specified by flag
	if flags, ok := r.conf.ProviderSpecificFlags[Name]; ok {
		if gcName := flags[GlobalConfigurationFlag]; gcName != "" {
			// Parse namespace/name format, fallback to provider config namespace
			namespace, name := parseNamespacedName(gcName, r.conf.Namespace)

			globalConfs, err := genericReadFromFile(
				filename,
				namespace,
				GlobalConfigurationGVK,
				newGlobalConfiguration,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to read GlobalConfigurations from file: %w", err)
			}
			for _, gc := range globalConfs {
				notify(notifications.WarningNotification, fmt.Sprintf("GlobalConfiguration name: %s", gc.Name), &gc)
				if gc.Name == name {
					storage.GlobalConfiguration = &gc
					break
				}
			}
		}
	}

	services, err := common.ReadServicesFromFile(filename, r.conf.Namespace)
	if err != nil {
		return nil, err
	}
	storage.ServicePorts = common.GroupServicePortsByPortName(services)

	return storage, nil
}
