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
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
)

// SuperpodReconciler reconciles a Superpod object.
type SuperpodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	ownerUID := sp.UID
	pod := resources.NewPod(sp)
	children := []client.Object{
		resources.NewServiceAccount(sp),
		resources.NewConfigMap(sp),
		pod,
		resources.NewService(sp),
		resources.NewIngress(sp),
	}
	for _, child := range children {
		terminating, err := resources.Ensure(ctx, r.Client, r.Scheme, sp, child)
		if err != nil {
			return ctrl.Result{}, err
		}
		if terminating {
			// A name cannot be reused until deletion finishes. Watches normally
			// wake us up; this short retry also covers a missed deletion event.
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	}

	// Re-fetch before updating status so concurrent edits produce a conflict
	// and retry rather than being overwritten. Avoid a write when unchanged.
	if err := r.Get(ctx, req.NamespacedName, sp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if sp.UID != ownerUID {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	if sp.DeletionTimestamp.IsZero() && sp.Status.PodName != pod.Name {
		sp.Status.PodName = pod.Name
		if err := r.Status().Update(ctx, sp); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
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
