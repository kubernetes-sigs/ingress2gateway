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

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

func createGatewayResources(
	ctx context.Context,
	l logger,
	client *gwclientset.Clientset,
	ns string,
	res []i2gw.GatewayResources,
	skipCleanup bool,
) (func(), error) {
	var cleanupFuncs []func()
	for _, r := range res {
		cleanup, err := createGatewayClasses(ctx, l, client, r.GatewayClasses, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating gateway classes: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createGateways(ctx, l, client, ns, r.Gateways, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating gateways: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createHTTPRoutes(ctx, l, client, ns, r.HTTPRoutes, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating http routes: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createGRPCRoutes(ctx, l, client, ns, r.GRPCRoutes, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating grpc routes: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createTLSRoutes(ctx, l, client, ns, r.TLSRoutes, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating tls routes: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createTCPRoutes(ctx, l, client, ns, r.TCPRoutes, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating tcp routes: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createUDPRoutes(ctx, l, client, ns, r.UDPRoutes, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating udp routes: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createBackendTLSPolicies(ctx, l, client, ns, r.BackendTLSPolicies, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating backend tls policies: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)

		cleanup, err = createReferenceGrants(ctx, l, client, ns, r.ReferenceGrants, skipCleanup)
		if err != nil {
			return nil, fmt.Errorf("creating reference grants: %w", err)
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			l.Logf("Skipping cleanup of gateway resources")
			return
		}
		for _, f := range cleanupFuncs {
			f()
		}
	}, nil
}

func createGateways(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, gws map[types.NamespacedName]v1.Gateway, skipCleanup bool) (func(), error) {
	for name, gw := range gws {
		// Ensure the namespace is set correctly.
		if gw.Namespace == "" {
			gw.Namespace = ns
		}

		y, err := toYAML(&gw)
		if err != nil {
			return nil, fmt.Errorf("converting gateway to YAML: %w", err)
		}

		l.Logf("Creating Gateway:\n%s", y)

		_, err = client.GatewayV1().Gateways(gw.Namespace).Create(
			ctx,
			&gw,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating Gateway %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, gw := range gws {
			namespace := gw.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting Gateway %s/%s", namespace, gw.Name)
			err := client.GatewayV1().Gateways(namespace).Delete(cleanupCtx, gw.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting Gateway %s: %v", gw.Name, err)
			}
		}
	}, nil
}

func createGatewayClasses(ctx context.Context, l logger, client *gwclientset.Clientset, gcs map[types.NamespacedName]v1.GatewayClass, skipCleanup bool) (func(), error) {
	for name, gc := range gcs {
		y, err := toYAML(&gc)
		if err != nil {
			return nil, fmt.Errorf("converting gateway class to YAML: %w", err)
		}

		l.Logf("Creating GatewayClass:\n%s", y)

		_, err = client.GatewayV1().GatewayClasses().Create(
			ctx,
			&gc,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating GatewayClass %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, gc := range gcs {
			log.Printf("Deleting GatewayClass %s", gc.Name)
			err := client.GatewayV1().GatewayClasses().Delete(cleanupCtx, gc.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting GatewayClass %s: %v", gc.Name, err)
			}
		}
	}, nil
}

func createHTTPRoutes(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1.HTTPRoute, skipCleanup bool) (func(), error) {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		if err != nil {
			return nil, fmt.Errorf("converting http route to YAML: %w", err)
		}

		l.Logf("Creating HTTPRoute:\n%s", y)

		_, err = client.GatewayV1().HTTPRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating HTTPRoute %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting HTTPRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1().HTTPRoutes(namespace).Delete(cleanupCtx, route.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting HTTPRoute %s: %v", route.Name, err)
			}
		}
	}, nil
}

func createGRPCRoutes(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1.GRPCRoute, skipCleanup bool) (func(), error) {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		if err != nil {
			return nil, fmt.Errorf("converting grpc route to YAML: %w", err)
		}

		l.Logf("Creating GRPCRoute:\n%s", y)

		_, err = client.GatewayV1().GRPCRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating GRPCRoute %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting GRPCRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1().GRPCRoutes(namespace).Delete(cleanupCtx, route.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting GRPCRoute %s: %v", route.Name, err)
			}
		}
	}, nil
}

func createTLSRoutes(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1alpha2.TLSRoute, skipCleanup bool) (func(), error) {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		if err != nil {
			return nil, fmt.Errorf("converting tls route to YAML: %w", err)
		}

		l.Logf("Creating TLSRoute:\n%s", y)

		_, err = client.GatewayV1alpha2().TLSRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating TLSRoute %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting TLSRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1alpha2().TLSRoutes(namespace).Delete(cleanupCtx, route.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting TLSRoute %s: %v", route.Name, err)
			}
		}
	}, nil
}

func createTCPRoutes(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1alpha2.TCPRoute, skipCleanup bool) (func(), error) {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		if err != nil {
			return nil, fmt.Errorf("converting tcp route to YAML: %w", err)
		}

		l.Logf("Creating TCPRoute:\n%s", y)

		_, err = client.GatewayV1alpha2().TCPRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating TCPRoute %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting TCPRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1alpha2().TCPRoutes(namespace).Delete(cleanupCtx, route.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting TCPRoute %s: %v", route.Name, err)
			}
		}
	}, nil
}

func createUDPRoutes(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1alpha2.UDPRoute, skipCleanup bool) (func(), error) {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		if err != nil {
			return nil, fmt.Errorf("converting udp route to YAML: %w", err)
		}

		l.Logf("Creating UDPRoute:\n%s", y)

		_, err = client.GatewayV1alpha2().UDPRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating UDPRoute %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting UDPRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1alpha2().UDPRoutes(namespace).Delete(cleanupCtx, route.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting UDPRoute %s: %v", route.Name, err)
			}
		}
	}, nil
}

func createBackendTLSPolicies(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, policies map[types.NamespacedName]v1.BackendTLSPolicy, skipCleanup bool) (func(), error) {
	for name, policy := range policies {
		// Ensure the namespace is set correctly.
		if policy.Namespace == "" {
			policy.Namespace = ns
		}

		y, err := toYAML(&policy)
		if err != nil {
			return nil, fmt.Errorf("converting backend tls policy to YAML: %w", err)
		}

		l.Logf("Creating BackendTLSPolicy:\n%s", y)

		_, err = client.GatewayV1().BackendTLSPolicies(policy.Namespace).Create(
			ctx,
			&policy,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating BackendTLSPolicy %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, policy := range policies {
			namespace := policy.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting BackendTLSPolicy %s/%s", namespace, policy.Name)
			err := client.GatewayV1().BackendTLSPolicies(namespace).Delete(cleanupCtx, policy.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting BackendTLSPolicy %s: %v", policy.Name, err)
			}
		}
	}, nil
}

func createReferenceGrants(ctx context.Context, l logger, client *gwclientset.Clientset, ns string, grants map[types.NamespacedName]v1beta1.ReferenceGrant, skipCleanup bool) (func(), error) {
	for name, grant := range grants {
		// Ensure the namespace is set correctly.
		if grant.Namespace == "" {
			grant.Namespace = ns
		}

		y, err := toYAML(&grant)
		if err != nil {
			return nil, fmt.Errorf("converting reference grant to YAML: %w", err)
		}

		l.Logf("Creating ReferenceGrant:\n%s", y)

		_, err = client.GatewayV1beta1().ReferenceGrants(grant.Namespace).Create(
			ctx,
			&grant,
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating ReferenceGrant %s: %w", name.String(), err)
		}
	}

	//nolint:contextcheck // Intentional background context in cleanup function
	return func() {
		if skipCleanup {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		for _, grant := range grants {
			namespace := grant.Namespace
			if namespace == "" {
				namespace = ns
			}
			log.Printf("Deleting ReferenceGrant %s/%s", namespace, grant.Name)
			err := client.GatewayV1beta1().ReferenceGrants(namespace).Delete(cleanupCtx, grant.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Deleting ReferenceGrant %s: %v", grant.Name, err)
			}
		}
	}, nil
}

// Extracts all Gateway resources from the specified GatewayResources slice and returns them as a
// map of namespaced names to Gateways.
func getGateways(res []i2gw.GatewayResources) map[types.NamespacedName]gwapiv1.Gateway {
	gateways := make(map[types.NamespacedName]gwapiv1.Gateway)

	for _, r := range res {
		for k, v := range r.Gateways {
			gateways[k] = v
		}
	}

	return gateways
}
