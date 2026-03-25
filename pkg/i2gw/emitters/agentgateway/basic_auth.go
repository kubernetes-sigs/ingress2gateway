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

package agentgateway

import (
	"fmt"

	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"github.com/kgateway-dev/ingress2gateway/pkg/i2gw/notifications"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"
	corev1 "k8s.io/api/core/v1"
)

// basicAuthSecretKey de-dupes notifications across routes/policies.
type basicAuthSecretKey struct {
	Namespace string
	Secret    string
}

// applyBasicAuthPolicy projects the BasicAuth IR policy into an AgentgatewayPolicy,
// returning true if it modified/created an AgentgatewayPolicy for the provided ingress.
//
// Semantics:
//   - If BasicAuth is configured, set spec.traffic.basicAuthentication.secretRef.name
//     to the referenced Secret.
//
// Note:
//   - Agentgateway expects the Secret to contain a key named '.htaccess' with htpasswd
//     content (per the BasicAuthentication API docs). The emitter cannot validate or
//     rewrite Secret contents at generation time.
func applyBasicAuthPolicy(
	pol emitterir.Policy,
	ingressName, namespace string,
	ap map[string]*agentgatewayv1alpha1.AgentgatewayPolicy,
) bool {
	if pol.BasicAuth == nil || pol.BasicAuth.SecretName == "" {
		return false
	}

	agp := ensureAgentgatewayPolicy(ap, ingressName, namespace)
	if agp.Spec.Traffic == nil {
		agp.Spec.Traffic = &agentgatewayv1alpha1.Traffic{}
	}

	agp.Spec.Traffic.BasicAuthentication = &agentgatewayv1alpha1.BasicAuthentication{
		SecretRef: &corev1.LocalObjectReference{
			Name: pol.BasicAuth.SecretName,
		},
	}

	ap[ingressName] = agp
	return true
}

// emitBasicAuthSecretNotifications emits an INFO notification whenever the agentgateway emitter
// projects BasicAuth into an AgentgatewayPolicy to warn users about theSecret key expectations.
func emitBasicAuthSecretNotifications(
	pol emitterir.Policy,
	sourceIngressName string,
	routeNamespace string,
	seen map[basicAuthSecretKey]struct{},
) {
	if pol.BasicAuth == nil || pol.BasicAuth.SecretName == "" {
		return
	}

	key := basicAuthSecretKey{
		Namespace: routeNamespace,
		Secret:    pol.BasicAuth.SecretName,
	}
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}

	msg := fmt.Sprintf(
		`Ingress %q uses Basic Auth (nginx.ingress.kubernetes.io/auth-type=basic) and references Secret %s/%s.

ingress2gateway projected this into an AgentgatewayPolicy:
  spec.traffic.basicAuthentication.secretRef.name: %q

IMPORTANT: Secret key expectations differ by dataplane:
  - agentgateway expects htpasswd content under key ".htaccess"
  - ingress-nginx (auth-file) commonly expects htpasswd content under key "auth"
`,
		sourceIngressName,
		routeNamespace,
		pol.BasicAuth.SecretName,
		pol.BasicAuth.SecretName,
	)

	notifications.NotificationAggr.DispatchNotification(
		notifications.NewNotification(notifications.InfoNotification, msg),
		"ingress-nginx",
	)
}
