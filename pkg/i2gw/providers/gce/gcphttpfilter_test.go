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
	"reflect"
	"testing"

	emittergce "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/emitter_intermediate/gce"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGCPHTTPFilter_DeepCopyObject(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	filter := &GCPHTTPFilter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-filter",
			Labels: map[string]string{
				"key": "value",
			},
		},
		Spec: GCPHTTPFilterSpec{
			CachePolicy: &emittergce.CachePolicy{
				CacheMode:         "CACHE_ALL_STATIC",
				RequestCoalescing: boolPtr(true),
				CacheKeyPolicy: &emittergce.CacheKeyPolicy{
					IncludeHost:             boolPtr(true),
					ExcludedQueryParameters: []string{"q1"},
				},
				NegativeCachingPolicy: []emittergce.NegativeCachingPolicy{
					{Code: 404, TTL: "300s"},
				},
				CacheBypassRequestHeaderNames: []string{"x-bypass"},
			},
		},
	}

	copyObj := filter.DeepCopyObject()
	copyFilter, ok := copyObj.(*GCPHTTPFilter)
	if !ok {
		t.Fatalf("DeepCopyObject returned type %T, want *GCPHTTPFilter", copyObj)
	}

	if filter == copyFilter {
		t.Errorf("DeepCopyObject returned same pointer")
	}

	if !reflect.DeepEqual(filter, copyFilter) {
		t.Errorf("DeepCopyObject content not equal: got %#v, want %#v", copyFilter, filter)
	}

	// Mutate original to verify deep copy
	filter.ObjectMeta.Labels["key"] = "new-value"
	if copyFilter.ObjectMeta.Labels["key"] == "new-value" {
		t.Errorf("ObjectMeta.Labels was shallow copied")
	}

	filter.Spec.CachePolicy.CacheMode = "NEW_MODE"
	if copyFilter.Spec.CachePolicy.CacheMode == "NEW_MODE" {
		t.Errorf("Spec.CachePolicy was shallow copied")
	}

	*filter.Spec.CachePolicy.CacheKeyPolicy.IncludeHost = false
	if *copyFilter.Spec.CachePolicy.CacheKeyPolicy.IncludeHost == false {
		t.Errorf("CacheKeyPolicy.IncludeHost was shallow copied")
	}

	filter.Spec.CachePolicy.CacheKeyPolicy.ExcludedQueryParameters[0] = "mutated"
	if copyFilter.Spec.CachePolicy.CacheKeyPolicy.ExcludedQueryParameters[0] == "mutated" {
		t.Errorf("CacheKeyPolicy.ExcludedQueryParameters was shallow copied")
	}

	filter.Spec.CachePolicy.NegativeCachingPolicy[0].Code = 500
	if copyFilter.Spec.CachePolicy.NegativeCachingPolicy[0].Code == 500 {
		t.Errorf("Spec.CachePolicy.NegativeCachingPolicy was shallow copied")
	}

	filter.Spec.CachePolicy.CacheBypassRequestHeaderNames[0] = "mutated"
	if copyFilter.Spec.CachePolicy.CacheBypassRequestHeaderNames[0] == "mutated" {
		t.Errorf("Spec.CachePolicy.CacheBypassRequestHeaderNames was shallow copied")
	}
}
