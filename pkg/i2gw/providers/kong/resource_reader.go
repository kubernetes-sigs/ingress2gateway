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
	"bytes"
	"context"
	"os"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configurationv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
	kongv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
)

// converter implements the i2gw.CustomResourceReader interface.
type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	return &resourceReader{
		conf: conf,
	}
}

func (r *resourceReader) ReadResourcesFromCluster(ctx context.Context, customResources map[schema.GroupVersionKind]interface{}) error {
	tcpIngressList := &kongv1beta1.TCPIngressList{}
	if err := r.conf.Client.List(ctx, tcpIngressList); err != nil {
		return err
	}
	if len(tcpIngressList.Items) > 0 {
		customResources[tcpIngressGVK] = tcpIngressList.Items
	}
	return nil
}

func (r *resourceReader) ReadResourcesFromFiles(ctx context.Context, customResources map[schema.GroupVersionKind]interface{}, filename string, namespace string) error {
	stream, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(stream)
	objs, err := i2gw.ExtractObjectsFromReader(reader)
	if err != nil {
		return err
	}

	tcpIngresses := []configurationv1beta1.TCPIngress{}
	for _, f := range objs {
		if namespace != "" && f.GetNamespace() != namespace {
			continue
		}
		if !f.GroupVersionKind().Empty() &&
			f.GroupVersionKind().Group == string(kongResourcesGroup) &&
			f.GroupVersionKind().Kind == string(kongTCPIngressKind) {
			var tcpIngress configurationv1beta1.TCPIngress
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(f.UnstructuredContent(), &tcpIngress)
			if err != nil {
				return err
			}
			tcpIngresses = append(tcpIngresses, tcpIngress)
		}
	}
	customResources[tcpIngressGVK] = tcpIngresses

	return nil
}
