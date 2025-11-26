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

package nginx

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/provider_intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

// gatewayResourcesConverter converts intermediate representation to Gateway API resources with NGINX-specific extensions
type gatewayResourcesConverter struct{}

// newGatewayResourcesConverter creates a new gateway resources converter
func newGatewayResourcesConverter() *gatewayResourcesConverter {
	return &gatewayResourcesConverter{}
}

// convert converts IR to Gateway API resources including NGINX Gateway Fabric custom policies
func (c *gatewayResourcesConverter) convert(ir provider_intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	// Start with standard Gateway API resources
	gatewayResources, errs := common.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	return gatewayResources, errs
}
