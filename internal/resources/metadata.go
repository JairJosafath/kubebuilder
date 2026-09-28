// Package resources builds the desired Kubernetes objects for a Superpod.
// Builders do not contact the API server. Pass a Superpod fetched from Kubernetes,
// so its namespace and server-assigned UID are available.
package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// ObjectKey returns the namespace and name that all of sp's children share.
func ObjectKey(sp *superv1.Superpod) types.NamespacedName {
	return types.NamespacedName{Namespace: sp.Namespace, Name: resourceName(sp)}
}

// resourceName is shared by children of different kinds. The UID keeps it stable
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
