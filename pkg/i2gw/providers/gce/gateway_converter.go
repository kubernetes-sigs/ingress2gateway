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

package gce

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type irToGatewayResourcesConverter struct{}

// newIRToGatewayResourcesConverter returns an gce irToGatewayResourcesConverter instance.
func newIRToGatewayResourcesConverter() irToGatewayResourcesConverter {
	return irToGatewayResourcesConverter{}
}

func (c *irToGatewayResourcesConverter) irToGateway(ir intermediate.IR) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources, errs := common.ToGatewayResources(ir)
	if len(errs) != 0 {
		return i2gw.GatewayResources{}, errs
	}
	buildGceServiceExtensions(ir, &gatewayResources)
	return gatewayResources, nil
}
