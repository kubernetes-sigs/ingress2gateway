/*
Copyright 2024 The Kubernetes Authors.

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

package i2gw

import (
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
)

func GetIngressClass(ingress networkingv1.Ingress) string {
	var ingressClass string

	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName != "" {
		ingressClass = *ingress.Spec.IngressClassName
	} else if _, ok := ingress.Annotations[networkingv1beta1.AnnotationIngressClass]; ok {
		ingressClass = ingress.Annotations[networkingv1beta1.AnnotationIngressClass]
	} else {
		ingressClass = ingress.Name
	}

	return ingressClass
}
