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

package nginx

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
)

const virtualServerManifests = `
apiVersion: k8s.nginx.org/v1
kind: VirtualServer
metadata:
  name: shop
  namespace: default
spec:
  host: shop.example.com
  tls:
    secret: shop-secret
    redirect:
      enable: true
  upstreams:
  - name: web
    service: web-svc
    port: 8080
  - name: api
    service: api-svc
    port: 80
  routes:
  - path: /
    action:
      pass: web
  - path: =/healthz
    action:
      pass: web
  - path: /api
    splits:
    - weight: 100
      action:
        pass: api
  - path: ~ ^/img/
    action:
      pass: web
  - path: /v2
    action:
      proxy:
        upstream: api
  - path: /old
    action:
      proxy:
        upstream: api
        rewritePath: /
  - path: =/
    action:
      redirect:
        url: https://shop.example.com/home
status:
  state: Valid
---
apiVersion: k8s.nginx.org/v1
kind: VirtualServer
metadata:
  name: internal
  namespace: default
spec:
  ingressClassName: nginx-internal
  host: internal.example.com
  upstreams:
  - name: web
    service: web-svc
    port: 80
  routes:
  - path: /
    action:
      pass: web
`

func TestVirtualServerToIR(t *testing.T) {
	report := notifications.NewReport(true)
	provider := NewProvider(&i2gw.ProviderConf{Report: report})
	require.NoError(t, provider.ReadResourcesFromFile(context.Background(), strings.NewReader(virtualServerManifests)))

	ir, errs := provider.ToIR()
	require.Empty(t, errs)

	require.Len(t, ir.HTTPRoutes, 1)
	route := ir.HTTPRoutes[types.NamespacedName{Namespace: "default", Name: "shop-shop-example-com"}]
	var rules []string
	for _, rule := range route.Spec.Rules {
		path, backend := rule.Matches[0].Path, rule.BackendRefs[0]
		rules = append(rules, fmt.Sprintf("%s %s -> %s:%d", *path.Type, *path.Value, backend.Name, *backend.Port))
	}
	require.Equal(t, []string{"PathPrefix / -> web-svc:8080", "Exact /healthz -> web-svc:8080", "PathPrefix /v2 -> api-svc:80"}, rules)

	require.Len(t, ir.Gateways, 1)
	listeners := ir.Gateways[types.NamespacedName{Namespace: "default", Name: "nginx"}].Spec.Listeners
	require.Len(t, listeners, 2)
	require.EqualValues(t, "shop.example.com", *listeners[1].Hostname)
	require.EqualValues(t, "shop-secret", listeners[1].TLS.CertificateRefs[0].Name)

	var warnings []string
	for _, n := range report.Notifications()[Name] {
		warnings = append(warnings, n.Message)
	}
	require.Equal(t, []string{
		"VirtualServer fields not converted: spec.routes[2].splits, spec.tls.redirect",
		"VirtualServer routes skipped, only prefix and exact paths that pass or proxy to a defined upstream without extra options are converted: /api, ~ ^/img/, /old, =/",
	}, warnings)
}

func TestReadVirtualServersFromClusterWithoutCRD(t *testing.T) {
	c := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return &meta.NoKindMatchError{GroupKind: virtualServerGVK.GroupKind()}
		},
	}).Build()

	got, err := readVirtualServersFromCluster(context.Background(), c, sets.New("nginx"))
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestVirtualServerIngressClassFlag(t *testing.T) {
	provider := NewProvider(&i2gw.ProviderConf{
		Report:                notifications.NewReport(true),
		ProviderSpecificFlags: map[string]map[string]string{Name: {NginxIngressClassFlag: "nginx-internal"}},
	})
	require.NoError(t, provider.ReadResourcesFromFile(context.Background(), strings.NewReader(virtualServerManifests)))

	ir, errs := provider.ToIR()
	require.Empty(t, errs)
	require.Len(t, ir.HTTPRoutes, 1)
	require.Contains(t, ir.HTTPRoutes, types.NamespacedName{Namespace: "default", Name: "internal-internal-example-com"})
}
