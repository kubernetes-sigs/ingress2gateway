/*
Copyright 2023 The Kubernetes Authors.

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

package kong

import (
	kongv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

type storage struct {
	Ingresses    map[types.NamespacedName]*networkingv1.Ingress
	TCPIngresses []kongv1beta1.TCPIngress
	ServicePorts map[types.NamespacedName]map[string]int32
}

func newResourceStorage() *storage {
	return &storage{
		Ingresses:    map[types.NamespacedName]*networkingv1.Ingress{},
		TCPIngresses: []kongv1beta1.TCPIngress{},
		ServicePorts: map[types.NamespacedName]map[string]int32{},
	}
}
