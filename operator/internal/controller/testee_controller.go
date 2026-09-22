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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	testeev1 "github.com/jairjosafath/operator/api/v1"
)

// TesteeReconciler reconciles a Testee object
type TesteeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=testee.elp-max.com,resources=testees,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=testee.elp-max.com,resources=testees/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=testee.elp-max.com,resources=testees/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Testee object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *TesteeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	l.Info("starting reconciliation", "Namespace", req.Namespace)
	// TODO(user): your logic here
	testee := &testeev1.Testee{}

	if err := r.Get(ctx, req.NamespacedName, testee); err != nil {
		if errors.IsNotFound(err) {
			l.Info("resource deleted, skipping reconsiliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if testee.Status.Phase == "" {
		l.Info("status.phase is not empty which means this is not a new object:", "Name", testee.Name)
		// checking if the state has changed
		if testee.Status.Phase != "state value retrieved from resource" {
			l.Info("the state of the resource has changed, updating the object accordingly")
			testee.Status.Phase = "new value from the resource"

			if err := r.Update(ctx, testee); err != nil {
				return ctrl.Result{}, nil
			}
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TesteeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testeev1.Testee{}).
		Named("testee").
		Complete(r)
}
