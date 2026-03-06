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

package notifications

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	InfoNotification    MessageType = "INFO"
	WarningNotification MessageType = "WARNING"
	ErrorNotification   MessageType = "ERROR"
)

type MessageType string

type Notification struct {
	Type           MessageType
	Message        string
	CallingObjects []client.Object
}

func objectsToStr(ob []client.Object) string {
	strs := make([]string, len(ob))

	for i, o := range ob {
		strs[i] = o.GetObjectKind().GroupVersionKind().Kind + ": " + client.ObjectKeyFromObject(o).String()
	}

	return strings.Join(strs, ", ")
}
