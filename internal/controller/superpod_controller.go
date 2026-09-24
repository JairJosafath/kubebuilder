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

package controller

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/resources"
)

// SuperpodReconciler reconciles a Superpod object.
type SuperpodReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EmojiSelector EmojiSelector
}

// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods,verbs=get;list;watch
// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods;configmaps;serviceaccounts;services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch

// Reconcile can run many times for the same Superpod. Each pass compares the
// desired resources with the cluster, rather than treating status as a flag
// meaning that creation is permanently finished.
func (r *SuperpodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	sp := &superv1.Superpod{}
	if err := r.Get(ctx, req.NamespacedName, sp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !sp.DeletionTimestamp.IsZero() {
		// Kubernetes garbage collection deletes children through owner references.
		// Do not recreate resources while their parent is being deleted.
		return ctrl.Result{}, nil
	}

	pod := resources.NewPod(sp)
	page, emojiErr := r.emojiPage(ctx, sp)
	children := []client.Object{
		resources.NewServiceAccount(sp),
		page,
		pod,
		resources.NewService(sp),
		resources.NewIngress(sp),
	}
	for _, child := range children {
		terminating, err := resources.Ensure(ctx, r.Client, r.Scheme, sp, child)
		if err != nil {
			statusErr := r.updateStatus(ctx, sp, sp.Status.PodName, metav1.Condition{
				Status: metav1.ConditionFalse, Reason: "ReconcileFailed", Message: err.Error(),
			})
			return ctrl.Result{}, errors.Join(err, statusErr)
		}
		if terminating {
			// A name cannot be reused until deletion finishes. Watches normally
			// wake us up; this short retry also covers a missed deletion event.
			return ctrl.Result{RequeueAfter: time.Second}, r.updateStatus(ctx, sp, sp.Status.PodName, metav1.Condition{
				Status: metav1.ConditionFalse, Reason: "ResourceTerminating",
				Message: "Waiting for a child resource to finish deletion before recreating it",
			})
		}
	}

	if emojiErr != nil {
		// The plain ability page remains available while Jev is unavailable.
		delay, reason := time.Minute, "EmojiSelectionFailed"
		if rateLimit, ok := errors.AsType[*emoji.RateLimitError](emojiErr); ok {
			delay, reason = max(time.Second, time.Until(rateLimit.RetryAt)), "EmojiRateLimited"
		}
		return ctrl.Result{RequeueAfter: delay}, r.updateStatus(ctx, sp, pod.Name, metav1.Condition{
			Status: metav1.ConditionFalse, Reason: reason, Message: emojiErr.Error(),
		})
	}

	condition, err := r.readiness(ctx, sp)
	if err != nil {
		statusErr := r.updateStatus(ctx, sp, pod.Name, metav1.Condition{
			Status: metav1.ConditionUnknown, Reason: "ObservationFailed", Message: err.Error(),
		})
		return ctrl.Result{}, errors.Join(err, statusErr)
	}
	return ctrl.Result{}, r.updateStatus(ctx, sp, pod.Name, condition)
}

// SetupWithManager watches the Superpod and every kind of child it owns.
func (r *SuperpodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&superv1.Superpod{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Named("superpod").
		Complete(r)
}
