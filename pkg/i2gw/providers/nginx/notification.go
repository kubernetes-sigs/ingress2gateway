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

package nginx

// import (
//	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"
//	"sigs.k8s.io/controller-runtime/pkg/client"
// )

// notify dispatches a notification with the nginx provider name
// Currently unused but kept for future notification needs
// func notify(mType notifications.MessageType, message string, callingObject ...client.Object) {
//	newNotification := notifications.NewNotification(mType, message, callingObject...)
//	notifications.NotificationAggr.DispatchNotification(newNotification, string(Name))
// }
