package kgateway

import (
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

type storage struct {
	Ingresses    map[types.NamespacedName]*networkingv1.Ingress
	Services     map[types.NamespacedName]*apiv1.Service
	ServicePorts map[types.NamespacedName]map[string]int32
}

func newResourcesStorage() *storage {
	return &storage{
		Ingresses:    map[types.NamespacedName]*networkingv1.Ingress{},
		Services:     map[types.NamespacedName]*apiv1.Service{},
		ServicePorts: map[types.NamespacedName]map[string]int32{},
	}
}
