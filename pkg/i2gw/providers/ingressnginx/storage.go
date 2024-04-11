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

package ingressnginx

import (
	"sort"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OrderedIngressMap struct {
	ingressNames   []types.NamespacedName
	ingressObjects map[types.NamespacedName]*networkingv1.Ingress
}
type storage struct {
	Ingresses OrderedIngressMap
}

func newResourcesStorage() *storage {
	return &storage{
		Ingresses: OrderedIngressMap{
			ingressNames:   []types.NamespacedName{},
			ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{},
		},
	}
}

func (oim *OrderedIngressMap) List() []networkingv1.Ingress {
	ingressList := []networkingv1.Ingress{}
	for _, ing := range oim.ingressNames {
		ingressList = append(ingressList, *oim.ingressObjects[ing])
	}
	return ingressList
}

func (oim *OrderedIngressMap) FromMap(ingresses map[types.NamespacedName]*networkingv1.Ingress) {
	ingNames := []types.NamespacedName{}
	for ing := range ingresses {
		ingNames = append(ingNames, ing)
	}
	sort.Slice(ingNames, func(i, j int) bool {
		return ingNames[i].Name < ingNames[j].Name
	})
	oim.ingressNames = ingNames
	oim.ingressObjects = ingresses
}
