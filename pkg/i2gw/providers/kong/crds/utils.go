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

package crds

import (
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func buildSectionName(parts ...string) *gatewayv1.SectionName {
	builder := strings.Builder{}
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i != 0 {
			builder.WriteString("-")
		}
		builder.WriteString(p)
	}
	return (*gatewayv1.SectionName)(common.PtrTo(builder.String()))
}
