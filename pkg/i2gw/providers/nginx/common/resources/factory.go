package resources

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx/common"
)

const (
	BackendTLSPolicyKind = "BackendTLSPolicy"
	GRPCRouteKind        = "GRPCRoute"
	ServiceKind          = "Service"
)

// ResourceType represents the type of resource to create
type ResourceType string

const (
	BackendTLSPolicyType ResourceType = "BackendTLSPolicy"
	GRPCRouteType        ResourceType = "GRPCRoute"
)

// BackendTLSPolicyOptions contains options for BackendTLSPolicy creation
type BackendTLSPolicyOptions struct {
	// Name of the policy
	Name string
	// Namespace of the policy
	Namespace string
	// Target service name
	ServiceName string
	// Source label for tracking the origin (e.g., "nginx-ssl-services")
	SourceLabel string
	// Labels to apply to the policy (additional to the source label)
	Labels map[string]string
}

// PolicyOptions contains all policy configuration options
type PolicyOptions struct {
	BackendTLS *BackendTLSPolicyOptions
	// NotificationCollector for gathering notifications during policy creation
	NotificationCollector common.NotificationCollector
	// Source object for notifications (e.g., VirtualServer, Ingress)
	SourceObject client.Object
}

// CreateBackendTLSPolicy creates a BackendTLSPolicy using the provided options
func CreateBackendTLSPolicy(opts PolicyOptions) *gatewayv1alpha3.BackendTLSPolicy {
	if opts.BackendTLS == nil {
		return nil
	}

	btlsOpts := opts.BackendTLS

	// Build labels
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "ingress2gateway",
	}
	if btlsOpts.SourceLabel != "" {
		labels["ingress2gateway.io/source"] = btlsOpts.SourceLabel
	}
	for k, v := range btlsOpts.Labels {
		labels[k] = v
	}

	policy := &gatewayv1alpha3.BackendTLSPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha3.GroupVersion.String(),
			Kind:       BackendTLSPolicyKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      btlsOpts.Name,
			Namespace: btlsOpts.Namespace,
			Labels:    labels,
		},
		Spec: gatewayv1alpha3.BackendTLSPolicySpec{
			TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReferenceWithSectionName{
				{
					LocalPolicyTargetReference: gatewayv1alpha2.LocalPolicyTargetReference{
						Group: gatewayv1.GroupName,
						Kind:  ServiceKind,
						Name:  gatewayv1.ObjectName(btlsOpts.ServiceName),
					},
				},
			},
			Validation: gatewayv1alpha3.BackendTLSPolicyValidation{
				// Note: WellKnownCACertificates and Hostname fields are intentionally left empty
				// These fields must be manually configured based on your backend service's TLS setup
			},
		},
	}

	// Add notification about manual configuration required
	if opts.NotificationCollector != nil {
		message := fmt.Sprintf("BackendTLSPolicy '%s' created but requires manual configuration. You must set the 'validation.hostname' field to match your backend service's TLS certificate hostname, and configure appropriate CA certificates or certificateRefs for TLS verification.", btlsOpts.Name)
		opts.NotificationCollector.AddWarning(message, opts.SourceObject)
	}

	return policy
}

// NewBackendTLSPolicyOptions creates BackendTLSPolicyOptions with common defaults
func NewBackendTLSPolicyOptions(name, namespace, serviceName, sourceLabel string) *BackendTLSPolicyOptions {
	return &BackendTLSPolicyOptions{
		Name:        name,
		Namespace:   namespace,
		ServiceName: serviceName,
		SourceLabel: sourceLabel,
		Labels:      make(map[string]string),
	}
}

// GenerateBackendTLSPolicyName generates a consistent policy name
func GenerateBackendTLSPolicyName(serviceName, suffix string) string {
	if suffix != "" {
		return fmt.Sprintf("%s-%s-backend-tls", serviceName, suffix)
	}
	return fmt.Sprintf("%s-backend-tls", serviceName)
}

// GeneratePolicyKey generates a NamespacedName key for policy storage
func GeneratePolicyKey(namespace, name string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}
