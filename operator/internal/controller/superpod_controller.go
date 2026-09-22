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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// SuperpodReconciler reconciles a Superpod object
type SuperpodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Superpod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *SuperpodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("############## Start Reconcile ############## ")

	superPod := &superv1.Superpod{}
	if err := r.Get(ctx, req.NamespacedName, superPod); err != nil {
		if errors.IsNotFound(err) {
			log.Info("superpod could not be found:", "name", superPod.Name, "namespace", req.Namespace)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err // since result is empty, this will trigger exp backoff loop
	}

	// the finalizer is actually the one who should create the resource

	log.Info("===== create new super pod ====")
	podName, err := createSuperPod(ctx, superPod.Spec.SuperAbility, req.Namespace, superPod.Name)
	if err != nil {
		log.Info("pod could not be created:", "name", superPod.Name, "namespace", req.Namespace)
		return reconcile.Result{}, err
	}

	log.Info("==== update status")
	superPod.Spec.PodName = podName


	log.Info("=== waiting for pod to be in the running state")
	

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SuperpodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&superv1.Superpod{}).
		Named("superpod").
		Complete(r)
}

func createSuperPod(ctx context.Context, superAbility, namespace, superPodName string) (string, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("failed to setup in-cluster configuration: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to setup in-cluster client: %w", err)
	}

	pod := &corev1.Pod{}
	pod.Name = namespace + "-" + superAbility
	pod.Labels = map[string]string{"parent": superPodName, "ability": superAbility}

	fmt.Printf("creating pod...")

	ok, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, v1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}

	fmt.Printf("Created pod %q.\n", ok.GetObjectMeta().GetName())

	return ok.Name, err
}
