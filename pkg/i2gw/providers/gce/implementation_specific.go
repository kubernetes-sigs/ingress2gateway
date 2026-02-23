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
	"fmt"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// implementationSpecificHTTPPathTypeMatch returns a converter which maps the
// Implementation Specific HTTP path and type to the corresponding Gateway HTTP ones.
//
// Ingress path with type `ImplementationSpecific` will:
// - Translate to equivalent Gateway Prefix path but dropping `/*`, if `/*` exists
// - Translate to equivalent Exact path otherwise.
// Example:
// | Ingress `ImplementationSpecific` Path | map to Gateway Path                    |
// | ------------------------------------- | -------------------------------------- |
// | /*                                    | / Prefix                               |
// | /v1                                   | /v1 Exact                              |
// | /v1/                                  | /v1/ Exact                             |
// | /v1/*                                 | /v1 Prefix                             |
func implementationSpecificHTTPPathTypeMatch(notify notifications.NotifyFunc) i2gw.ImplementationSpecificHTTPPathTypeMatchConverter {
	return func(path *gatewayv1.HTTPPathMatch) {
		pmExact := gatewayv1.PathMatchExact
		pmPrefix := gatewayv1.PathMatchPathPrefix

		if *path.Value == "/*" {
			path.Type = &pmPrefix
			path.Value = common.PtrTo("/")
			return
		}
		if !strings.HasSuffix(*path.Value, "/*") {
			path.Type = &pmExact
			return
		}

		currentValue := *path.Value
		path.Type = &pmPrefix
		path.Value = common.PtrTo(strings.TrimSuffix(*path.Value, "/*"))
		notify(notifications.WarningNotification, fmt.Sprintf("After conversion, ImplementationSpecific Path %s/* will additionally map to %s. See https://github.com/kubernetes-sigs/ingress2gateway/blob/main/pkg/i2gw/providers/gce/README.md for details.", currentValue, *path.Value))
		klog.Warningf("After conversion, ImplementationSpecific Path %s/* will additionally map to %s. See https://github.com/kubernetes-sigs/ingress2gateway/blob/main/pkg/i2gw/providers/gce/README.md for details.", currentValue, *path.Value)
	}
}
