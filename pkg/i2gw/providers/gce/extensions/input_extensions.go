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

package extensions

import (
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
	frontendconfigv1beta1 "k8s.io/ingress-gce/pkg/apis/frontendconfig/v1beta1"
)

func ValidateBeConfig(beConfig *backendconfigv1.BackendConfig) error {
	if beConfig.Spec.SessionAffinity != nil {
		if err := validateSessionAffinity(beConfig); err != nil {
			return err
		}
	}

	return nil
}

func validateSessionAffinity(beConfig *backendconfigv1.BackendConfig) error {
	if beConfig.Spec.SessionAffinity.AffinityCookieTtlSec != nil && beConfig.Spec.SessionAffinity.AffinityType != "GENERATED_COOKIE" {
		return fmt.Errorf("BackendConfig has affinityCookieTtlSec set, but affinityType is not GENERATED_COOKIE")
	}
	return nil
}

func BuildIRSessionAffinityConfig(beConfig *backendconfigv1.BackendConfig) *intermediate.SessionAffinityConfig {
	return &intermediate.SessionAffinityConfig{
		AffinityType: beConfig.Spec.SessionAffinity.AffinityType,
		CookieTTLSec: beConfig.Spec.SessionAffinity.AffinityCookieTtlSec,
	}
}

func BuildIRSecurityPolicyConfig(beConfig *backendconfigv1.BackendConfig) *intermediate.SecurityPolicyConfig {
	return &intermediate.SecurityPolicyConfig{
		Name: beConfig.Spec.SecurityPolicy.Name,
	}
}

func BuildIRSslPolicyConfig(feConfig *frontendconfigv1beta1.FrontendConfig) *intermediate.SslPolicyConfig {
	return &intermediate.SslPolicyConfig{
		Name: *feConfig.Spec.SslPolicy,
	}
}
