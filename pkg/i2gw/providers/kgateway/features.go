package kgateway

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func bufferFeature(ing2pol map[string]intermediate.KgatewayPolicy) func(ingresses []networkingv1.Ingress, svcPorts map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
	return func(ingresses []networkingv1.Ingress, svcPorts map[types.NamespacedName]map[string]int32, ir *intermediate.IR) field.ErrorList {
		var errList field.ErrorList
		for _, ingress := range ingresses {
			if buffersize := ingress.Annotations["nginx.ingress.kubernetes.io/client-body-buffer-size"]; buffersize != "" {
				buffer, err := resource.ParseQuantity(buffersize)
				if err != nil {
					errList = append(errList, field.Invalid(field.NewPath("metadata").Child("annotations").Key("nginx.ingress.kubernetes.io/client-body-buffer-size"), buffersize, "invalid buffer size"))
				}
				pol := ing2pol[ingress.Name]
				pol.Buffer = &buffer
				ing2pol[ingress.Name] = pol
			}
		}
		return errList
	}
}
