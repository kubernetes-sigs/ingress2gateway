/*
Copyright The Kubernetes Authors.

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

package traefik

import (
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const traefikAnnotationPrefix = "traefik.ingress.kubernetes.io/"

// supportedAnnotations is the set of Traefik annotations this provider converts.
// Any annotation with the Traefik prefix that is NOT in this set triggers a warning.
var supportedAnnotations = map[string]bool{
	RouterTLSAnnotation:         true,
	RouterEntrypointsAnnotation: true,
}

// unsupportedAnnotationsFeature emits a warning for every Traefik annotation that
// the provider does not convert, so users know they need to handle it manually.
func unsupportedAnnotationsFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, _ *providerir.ProviderIR) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			ing := rule.Ingress
			for annotation, val := range ing.Annotations {
				if !strings.HasPrefix(annotation, traefikAnnotationPrefix) {
					continue
				}
				if !supportedAnnotations[annotation] {
					notify(
						notifications.WarningNotification,
						fmt.Sprintf(
							"annotation %q (value: %q) on ingress %s/%s is not supported by the Traefik provider and will not be converted. "+
								"Please review the Gateway API equivalent manually.",
							annotation, val, ing.Namespace, ing.Name,
						),
					)
				}
			}
		}
	}
	return nil
}
