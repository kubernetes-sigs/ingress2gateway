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
	"reflect"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	backendconfigv1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1"
)

func TestBuildIRCdnConfig(t *testing.T) {
	tests := []struct {
		name     string
		beConfig *backendconfigv1.BackendConfig
		want     *gce.CdnConfig
	}{
		{
			name:     "nil cdn config",
			beConfig: &backendconfigv1.BackendConfig{Spec: backendconfigv1.BackendConfigSpec{}},
			want:     nil,
		},
		{
			name: "disabled cdn config",
			beConfig: &backendconfigv1.BackendConfig{
				Spec: backendconfigv1.BackendConfigSpec{
					Cdn: &backendconfigv1.CDNConfig{
						Enabled: false,
					},
				},
			},
			want: nil,
		},
		{
			name: "full cdn config",
			beConfig: &backendconfigv1.BackendConfig{
				Spec: backendconfigv1.BackendConfigSpec{
					Cdn: &backendconfigv1.CDNConfig{
						Enabled:                     true,
						CacheMode:                   ptr("CACHE_ALL_STATIC"),
						DefaultTtl:                  ptr(int64(3600)),
						MaxTtl:                      ptr(int64(7200)),
						ClientTtl:                   ptr(int64(1800)),
						RequestCoalescing:           ptr(true),
						ServeWhileStale:             ptr(int64(60)),
						NegativeCaching:             ptr(true),
						NegativeCachingPolicy:       []*backendconfigv1.NegativeCachingPolicy{{Code: 404, Ttl: 300}},
						BypassCacheOnRequestHeaders: []*backendconfigv1.BypassCacheOnRequestHeader{{HeaderName: "x-bypass"}},
						CachePolicy: &backendconfigv1.CacheKeyPolicy{
							IncludeHost:          true,
							IncludeProtocol:      true,
							IncludeQueryString:   true,
							QueryStringWhitelist: []string{"q"},
						},
					},
				},
			},
			want: &gce.CdnConfig{
				CachePolicy: &gce.CachePolicy{
					CacheMode:                     "CACHE_ALL_STATIC",
					DefaultTTL:                    "3600s",
					MaxTTL:                        "7200s",
					ClientTTL:                     "1800s",
					RequestCoalescing:             ptr(true),
					ServeWhileStale:               "60s",
					NegativeCaching:               ptr(true),
					NegativeCachingPolicy:         []gce.NegativeCachingPolicy{{Code: 404, TTL: "300s"}},
					CacheBypassRequestHeaderNames: []string{"x-bypass"},
					CacheKeyPolicy: &gce.CacheKeyPolicy{
						IncludeHost:             ptr(true),
						IncludeProtocol:         ptr(true),
						IncludeQueryString:      ptr(true),
						IncludedQueryParameters: []string{"q"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildIRCdnConfig(tt.beConfig)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildIRCdnConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
