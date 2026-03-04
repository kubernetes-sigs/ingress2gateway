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

package kgateway

import (
	"fmt"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/notifications"

	"k8s.io/apimachinery/pkg/types"
)

// backendProtoPatchKey de-dupes notifications across rules/routes/policies.
type backendProtoPatchKey struct {
	Namespace   string
	Service     string
	Port        int32
	AppProtocol string
}

// emitBackendProtocolPatchNotifications emits an INFO notification with the correct kubectl patch
// command to set ServicePort.appProtocol on an existing Service.
//
// IMPORTANT:
//   - We intentionally do NOT emit a Service object to avoid overwriting user-managed Service config.
//   - We also skip backends that have been rewritten to a kgateway Backend (service-upstream case),
//     because the generated Backend will carry appProtocol instead.
func emitBackendProtocolPatchNotifications(
	pol emitterir.Policy,
	sourceIngressName string,
	httpRouteKey types.NamespacedName,
	httpCtx emitterir.HTTPRouteContext,
	seen map[backendProtoPatchKey]struct{},
) {
	if pol.BackendProtocol == nil {
		return
	}

	// Map ingress-nginx backend-protocol → ServicePort.appProtocol
	var appProto string
	switch *pol.BackendProtocol {
	case emitterir.BackendProtocolGRPC:
		appProto = "grpc"
	default:
		// Nothing to do for unsupported/unknown mappings.
		return
	}

	for _, idx := range pol.RuleBackendSources {
		if idx.Rule >= len(httpCtx.Spec.Rules) {
			continue
		}
		rule := httpCtx.Spec.Rules[idx.Rule]
		if idx.Backend >= len(rule.BackendRefs) {
			continue
		}

		br := rule.BackendRefs[idx.Backend]

		// If already rewritten to a kgateway Backend, skip; appProtocol will be applied there.
		if br.BackendRef.Group != nil && *br.BackendRef.Group != "" {
			continue
		}
		if br.BackendRef.Kind != nil && *br.BackendRef.Kind != "" && *br.BackendRef.Kind != "Service" {
			continue
		}
		if br.BackendRef.Name == "" || br.BackendRef.Port == nil {
			continue
		}

		svcName := string(br.BackendRef.Name)
		// If service-upstream is enabled for this backend, the emitter will generate a
		// kgateway Backend with appProtocol, so do not suggest patching the Service.
		if len(pol.Backends) > 0 {
			if _, ok := pol.Backends[backendKeyForService(httpRouteKey.Namespace, svcName)]; ok {
				continue
			}
		}

		port := int32(*br.BackendRef.Port)

		key := backendProtoPatchKey{
			Namespace:   httpRouteKey.Namespace,
			Service:     svcName,
			Port:        port,
			AppProtocol: appProto,
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		// Use strategic merge patch (safe for core types) so only the matching port entry is updated.
		// This is far safer than emitting a Service manifest that users might blindly apply.
		patch := fmt.Sprintf(`{"spec":{"ports":[{"port":%d,"appProtocol":"%s"}]}}`, port, appProto)
		cmd := fmt.Sprintf(
			"kubectl patch service %s -n %s --type=strategic -p '%s'",
			svcName,
			httpRouteKey.Namespace,
			patch,
		)

		msg := fmt.Sprintf(
			`Ingress %q uses nginx.ingress.kubernetes.io/backend-protocol=%q for Service %s/%s port %d.

This annotation controls upstream protocol selection only, so ingress2gateway keeps HTTPRoute resources as HTTPRoute (it does not emit a GRPCRoute).

To avoid overwriting existing Service configuration, ingress2gateway does not emit a Service for this annotation.
Apply the equivalent behavior by patching your existing Service port's appProtocol:

  %s`,
			sourceIngressName,
			*pol.BackendProtocol,
			httpRouteKey.Namespace,
			svcName,
			port,
			cmd,
		)

		notifications.NotificationAggr.DispatchNotification(
			notifications.NewNotification(notifications.InfoNotification, msg),
			"ingress-nginx",
		)
	}
}
