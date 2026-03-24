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

package traefik

import (
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	providerir "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// unsupportedAnnotationsFeature emits warning notifications for Traefik annotations
// that have no direct Gateway API equivalent. This ensures users are aware of features
// that require manual migration.
func unsupportedAnnotationsFeature(notify notifications.NotifyFunc, ingresses []networkingv1.Ingress, _ map[types.NamespacedName]map[string]int32, _ *providerir.ProviderIR) field.ErrorList {
	unsupported := []struct {
		annotation string
		hint       string
	}{
		{
			RouterMiddlewaresAnnotation,
			"Traefik Middlewares have no direct Gateway API equivalent. " +
				"Consider using implementation-specific policy attachments (e.g. ExtensionRef filters) " +
				"supported by your Gateway implementation.",
		},
		{
			RouterPriorityAnnotation,
			"Traefik router priority has no direct Gateway API equivalent. " +
				"Gateway API uses rule ordering within an HTTPRoute for match precedence.",
		},
	}

	ruleGroups := common.GetRuleGroups(ingresses)
	for _, rg := range ruleGroups {
		for _, rule := range rg.Rules {
			ing := rule.Ingress
			for _, u := range unsupported {
				if val, ok := ing.Annotations[u.annotation]; ok {
					notify(
						notifications.WarningNotification,
						fmt.Sprintf(
							"annotation %q (value: %q) on ingress %s/%s cannot be automatically converted: %s",
							u.annotation, val,
							ing.Namespace, ing.Name,
							u.hint,
						),
					)
				}
			}
		}
	}
	return nil
}
