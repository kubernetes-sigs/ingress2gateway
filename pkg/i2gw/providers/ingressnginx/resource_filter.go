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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var filteredObjects = []schema.GroupKind{}

func init() {
	filteredObjects = append(filteredObjects, i2gw.DefaultFilteredObjects...)
}

// resourceReader implements the i2gw.resourceFilter interface.
type resourceFilter struct {
	conf *i2gw.ProviderConf
}

// newResourceFilter returns a resourceFilter instance.
func newResourceFilter(conf *i2gw.ProviderConf) *resourceFilter {
	return &resourceFilter{
		conf: conf,
	}
}

func (f *resourceFilter) Filter(objects []*unstructured.Unstructured) []*unstructured.Unstructured {
	filteredObjects := []*unstructured.Unstructured{}

	for _, o := range objects {
		var filterOut bool
		for _, f := range f.conf.FilteredObjects {
			if o.GetObjectKind().GroupVersionKind().Group == f.Group &&
				o.GetObjectKind().GroupVersionKind().Kind == f.Kind {
				filterOut = true
				break
			}
		}
		if !filterOut {
			filteredObjects = append(filteredObjects, o)
		}
	}

	return filteredObjects
}
