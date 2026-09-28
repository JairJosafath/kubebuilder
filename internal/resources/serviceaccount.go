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
