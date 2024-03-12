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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	annotationPrefix = "konghq.com"

	headersKey = "headers"
	methodsKey = "methods"
	pluginsKey = "plugins"
)

const (
	v1beta1Version = "v1beta1"

	kongResourcesGroup = "configuration.konghq.com"

	kongPluginKind = "KongPlugin"
	tcpIngressKind = "TCPIngress"
)

var (
	tcpIngressGVK = schema.GroupVersionKind{
		Group:   kongResourcesGroup,
		Version: v1beta1Version,
		Kind:    tcpIngressKind,
	}
)

func kongAnnotation(suffix string) string {
	return fmt.Sprintf("%s/%s", annotationPrefix, suffix)
}
