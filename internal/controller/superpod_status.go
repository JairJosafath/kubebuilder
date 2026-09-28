package controller

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
)

// readiness reads the Pod and Ingress and lets resources.Readiness judge them.
func (r *SuperpodReconciler) readiness(ctx context.Context, sp *superv1.Superpod) (metav1.Condition, error) {
	key := resources.ObjectKey(sp)
	pod, err := getIfExists(ctx, r, key, &corev1.Pod{})
	if err != nil {
		return metav1.Condition{}, err
	}
	ingress, err := getIfExists(ctx, r, key, &networkingv1.Ingress{})
	if err != nil {
		return metav1.Condition{}, err
	}
	return resources.Readiness(sp, pod, ingress), nil
}

// updateStatus re-fetches the Superpod and only writes when its status changes.
// A condition's observedGeneration ties the report to the spec we reconciled.
func (r *SuperpodReconciler) updateStatus(ctx context.Context, observed *superv1.Superpod, podName string, condition metav1.Condition) error {
	current := &superv1.Superpod{}

	if err := r.Get(ctx, client.ObjectKeyFromObject(observed), current); err != nil {
		return client.IgnoreNotFound(err)
	}

	if !current.DeletionTimestamp.IsZero() {
		return nil
	}

	if current.UID != observed.UID || current.Generation != observed.Generation {
		return apierrors.NewConflict(superv1.SchemeGroupVersion.WithResource("superpods").GroupResource(),
			current.Name, errors.New("superpod changed during reconciliation; retrying with its latest spec"))
	}

	before := current.DeepCopy()
	current.Status.PodName = podName
	current.Status.URL = "http://" + observed.Spec.Host
	condition.Type = superv1.ReadyCondition
	condition.ObservedGeneration = observed.Generation
	// SetStatusCondition preserves LastTransitionTime when the status is unchanged.
	meta.SetStatusCondition(&current.Status.Conditions, condition)
	if equality.Semantic.DeepEqual(before.Status, current.Status) {
		return nil
	}

	return r.Status().Update(ctx, current)
}
