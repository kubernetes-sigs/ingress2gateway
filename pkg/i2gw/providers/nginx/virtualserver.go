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
	"io"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx/annotations"
)

var virtualServerGVK = schema.GroupVersionKind{Group: "k8s.nginx.org", Version: "v1", Kind: "VirtualServer"}

type virtualServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              vsSpec `json:"spec"`
}

type vsSpec struct {
	IngressClassName string       `json:"ingressClassName"`
	Host             string       `json:"host"`
	TLS              vsTLS        `json:"tls"`
	Upstreams        []vsUpstream `json:"upstreams"`
	Routes           []vsRoute    `json:"routes"`
}

type vsTLS struct {
	Secret string `json:"secret"`
}

type vsUpstream struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Port    uint16 `json:"port"`
}

type vsRoute struct {
	Path   string   `json:"path"`
	Action vsAction `json:"action"`
}

type vsAction struct {
	Pass  string         `json:"pass"`
	Proxy map[string]any `json:"proxy"`
}

func readVirtualServersFromCluster(ctx context.Context, c client.Client, ingressClasses sets.Set[string]) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(virtualServerGVK)
	if err := c.List(ctx, list); err != nil && !meta.IsNoMatchError(err) {
		return nil, fmt.Errorf("failed to list VirtualServers: %w", err)
	}
	var virtualServers []unstructured.Unstructured
	for _, obj := range list.Items {
		if ingressClasses.Has(virtualServerIngressClass(&obj)) {
			virtualServers = append(virtualServers, obj)
		}
	}
	return virtualServers, nil
}

func readVirtualServersFromFile(reader io.Reader, namespace string, ingressClasses sets.Set[string]) ([]unstructured.Unstructured, error) {
	objs, err := common.ExtractObjectsFromReader(reader, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}
	var virtualServers []unstructured.Unstructured
	for _, obj := range objs {
		if obj.GroupVersionKind() == virtualServerGVK && ingressClasses.Has(virtualServerIngressClass(obj)) {
			virtualServers = append(virtualServers, *obj)
		}
	}
	return virtualServers, nil
}

func virtualServerIngressClass(obj *unstructured.Unstructured) string {
	class, _, _ := unstructured.NestedString(obj.Object, "spec", "ingressClassName")
	if class == "" {
		return annotations.NginxIngressClass
	}
	return class
}

func virtualServerToIngress(notify notifications.NotifyFunc, obj *unstructured.Unstructured) (*networkingv1.Ingress, error) {
	var vs virtualServer
	err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.UnstructuredContent(), &vs, true)
	strictErr, isStrict := runtime.AsStrictDecodingError(err)
	if err != nil && !isStrict {
		return nil, fmt.Errorf("failed to parse VirtualServer %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}

	if isStrict {
		var ignored []string
		for _, e := range strictErr.Errors() {
			if f := strings.Trim(strings.TrimPrefix(e.Error(), "unknown field "), `"`); strings.HasPrefix(f, "spec.") && !strings.Contains(f, ".action.") {
				ignored = append(ignored, f)
			}
		}
		if len(ignored) > 0 {
			notify(notifications.WarningNotification, "VirtualServer fields not converted: "+strings.Join(ignored, ", "), obj)
		}
	}

	backends := map[string]networkingv1.IngressBackend{}
	for _, u := range vs.Spec.Upstreams {
		backends[u.Name] = networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: u.Service,
				Port: networkingv1.ServiceBackendPort{Number: int32(u.Port)},
			},
		}
	}

	var paths []networkingv1.HTTPIngressPath
	var skipped []string
	for _, route := range vs.Spec.Routes {
		pathType := networkingv1.PathTypePrefix
		path, exact := strings.CutPrefix(route.Path, "=")
		if exact {
			pathType = networkingv1.PathTypeExact
		}
		upstream := route.Action.Pass
		if u, plain := route.Action.Proxy["upstream"].(string); plain && len(route.Action.Proxy) == 1 {
			upstream = u
		}
		backend, ok := backends[upstream]
		if !ok || !strings.HasPrefix(path, "/") {
			skipped = append(skipped, route.Path)
			continue
		}
		paths = append(paths, networkingv1.HTTPIngressPath{Path: path, PathType: &pathType, Backend: backend})
	}
	if len(skipped) > 0 {
		notify(notifications.WarningNotification, "VirtualServer routes skipped, only prefix and exact paths that pass or proxy to a defined upstream without extra options are converted: "+strings.Join(skipped, ", "), obj)
	}
	if len(paths) == 0 {
		return nil, nil
	}

	ingress := &networkingv1.Ingress{
		TypeMeta:   vs.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{Name: vs.Name, Namespace: vs.Namespace},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(virtualServerIngressClass(obj)),
			Rules: []networkingv1.IngressRule{{
				Host:             vs.Spec.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths}},
			}},
		},
	}
	if vs.Spec.TLS.Secret != "" {
		ingress.Spec.TLS = []networkingv1.IngressTLS{{Hosts: []string{vs.Spec.Host}, SecretName: vs.Spec.TLS.Secret}}
	}
	return ingress, nil
}
