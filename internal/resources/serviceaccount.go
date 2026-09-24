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

package resources

import (
	corev1 "k8s.io/api/core/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// NewServiceAccount gives the Pod its own identity without granting API access.
// Nginx reads mounted files, so it does not need an API token.
func NewServiceAccount(sp *superv1.Superpod) *corev1.ServiceAccount {
	automountToken := false
	return &corev1.ServiceAccount{
		ObjectMeta:                   metadata(sp),
		AutomountServiceAccountToken: &automountToken,
	}
}
