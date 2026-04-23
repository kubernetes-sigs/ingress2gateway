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
	emittergce "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// GCPHTTPFilter is the CRD for Cloud CDN configuration.
type GCPHTTPFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GCPHTTPFilterSpec `json:"spec,omitempty"`
}

type GCPHTTPFilterSpec struct {
	CachePolicy *emittergce.CachePolicy `json:"cachePolicy,omitempty"`
}

// DeepCopyObject implements the runtime.Object interface.
func (in *GCPHTTPFilter) DeepCopyObject() runtime.Object {
	out := new(GCPHTTPFilter)
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec.CachePolicy != nil {
		out.Spec.CachePolicy = new(emittergce.CachePolicy)
		*out.Spec.CachePolicy = *in.Spec.CachePolicy

		if in.Spec.CachePolicy.RequestCoalescing != nil {
			out.Spec.CachePolicy.RequestCoalescing = new(bool)
			*out.Spec.CachePolicy.RequestCoalescing = *in.Spec.CachePolicy.RequestCoalescing
		}
		if in.Spec.CachePolicy.NegativeCaching != nil {
			out.Spec.CachePolicy.NegativeCaching = new(bool)
			*out.Spec.CachePolicy.NegativeCaching = *in.Spec.CachePolicy.NegativeCaching
		}
		if in.Spec.CachePolicy.CacheKeyPolicy != nil {
			out.Spec.CachePolicy.CacheKeyPolicy = new(emittergce.CacheKeyPolicy)
			*out.Spec.CachePolicy.CacheKeyPolicy = *in.Spec.CachePolicy.CacheKeyPolicy

			if in.Spec.CachePolicy.CacheKeyPolicy.IncludeProtocol != nil {
				out.Spec.CachePolicy.CacheKeyPolicy.IncludeProtocol = new(bool)
				*out.Spec.CachePolicy.CacheKeyPolicy.IncludeProtocol = *in.Spec.CachePolicy.CacheKeyPolicy.IncludeProtocol
			}
			if in.Spec.CachePolicy.CacheKeyPolicy.IncludeHost != nil {
				out.Spec.CachePolicy.CacheKeyPolicy.IncludeHost = new(bool)
				*out.Spec.CachePolicy.CacheKeyPolicy.IncludeHost = *in.Spec.CachePolicy.CacheKeyPolicy.IncludeHost
			}
			if in.Spec.CachePolicy.CacheKeyPolicy.IncludeQueryString != nil {
				out.Spec.CachePolicy.CacheKeyPolicy.IncludeQueryString = new(bool)
				*out.Spec.CachePolicy.CacheKeyPolicy.IncludeQueryString = *in.Spec.CachePolicy.CacheKeyPolicy.IncludeQueryString
			}

			if in.Spec.CachePolicy.CacheKeyPolicy.ExcludedQueryParameters != nil {
				out.Spec.CachePolicy.CacheKeyPolicy.ExcludedQueryParameters = make([]string, len(in.Spec.CachePolicy.CacheKeyPolicy.ExcludedQueryParameters))
				copy(out.Spec.CachePolicy.CacheKeyPolicy.ExcludedQueryParameters, in.Spec.CachePolicy.CacheKeyPolicy.ExcludedQueryParameters)
			}
			if in.Spec.CachePolicy.CacheKeyPolicy.IncludedQueryParameters != nil {
				out.Spec.CachePolicy.CacheKeyPolicy.IncludedQueryParameters = make([]string, len(in.Spec.CachePolicy.CacheKeyPolicy.IncludedQueryParameters))
				copy(out.Spec.CachePolicy.CacheKeyPolicy.IncludedQueryParameters, in.Spec.CachePolicy.CacheKeyPolicy.IncludedQueryParameters)
			}
			if in.Spec.CachePolicy.CacheKeyPolicy.IncludedHeaderNames != nil {
				out.Spec.CachePolicy.CacheKeyPolicy.IncludedHeaderNames = make([]string, len(in.Spec.CachePolicy.CacheKeyPolicy.IncludedHeaderNames))
				copy(out.Spec.CachePolicy.CacheKeyPolicy.IncludedHeaderNames, in.Spec.CachePolicy.CacheKeyPolicy.IncludedHeaderNames)
			}
			if in.Spec.CachePolicy.CacheKeyPolicy.IncludedCookieNames != nil {
				out.Spec.CachePolicy.CacheKeyPolicy.IncludedCookieNames = make([]string, len(in.Spec.CachePolicy.CacheKeyPolicy.IncludedCookieNames))
				copy(out.Spec.CachePolicy.CacheKeyPolicy.IncludedCookieNames, in.Spec.CachePolicy.CacheKeyPolicy.IncludedCookieNames)
			}
		}
		if in.Spec.CachePolicy.NegativeCachingPolicy != nil {
			out.Spec.CachePolicy.NegativeCachingPolicy = make([]emittergce.NegativeCachingPolicy, len(in.Spec.CachePolicy.NegativeCachingPolicy))
			copy(out.Spec.CachePolicy.NegativeCachingPolicy, in.Spec.CachePolicy.NegativeCachingPolicy)
		}
		if in.Spec.CachePolicy.CacheBypassRequestHeaderNames != nil {
			out.Spec.CachePolicy.CacheBypassRequestHeaderNames = make([]string, len(in.Spec.CachePolicy.CacheBypassRequestHeaderNames))
			copy(out.Spec.CachePolicy.CacheBypassRequestHeaderNames, in.Spec.CachePolicy.CacheBypassRequestHeaderNames)
		}
	}
	return out
}
