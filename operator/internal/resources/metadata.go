/*
Copyright 2026.

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

// Package resources builds the desired Kubernetes objects for a Superpod.
// Builders do not contact the API server. Pass a Superpod fetched from Kubernetes,
// so its namespace and server-assigned UID are available.
package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// Different resource kinds may share a name. The UID keeps this name stable
// across reconciliations and short enough for a Service's 63-character limit.
func resourceName(sp *superv1.Superpod) string {
	return "superpod-" + string(sp.UID)
}

func podSelector(sp *superv1.Superpod) map[string]string {
	return map[string]string{
		"super.elp-max.com/superpod-uid": string(sp.UID),
	}
}

func metadata(sp *superv1.Superpod) metav1.ObjectMeta {
	labels := podSelector(sp)
	labels["app.kubernetes.io/name"] = "superpod"
	labels["app.kubernetes.io/managed-by"] = "superpod-controller"

	return metav1.ObjectMeta{
		Name:      resourceName(sp),
		Namespace: sp.Namespace,
		Labels:    labels,
	}
}
