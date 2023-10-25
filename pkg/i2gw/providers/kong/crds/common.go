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
	kongv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
)

type ruleGroupKey string

type tcpIngressAggregator struct {
	ruleGroups map[ruleGroupKey]*tcpIngressRuleGroup
}

type tcpIngressRuleGroup struct {
	namespace    string
	name         string
	ingressClass string
	host         string
	port         int
	tls          []kongv1beta1.IngressTLS
	rules        []ingressRule
}

type ingressRule struct {
	rule kongv1beta1.IngressRule
}
