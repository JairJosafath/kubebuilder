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

// readiness describes Kubernetes signals, not an end-to-end browser check.
// The Pod probe checks nginx; the Ingress controller publishes its address.
func (r *SuperpodReconciler) readiness(ctx context.Context, sp *superv1.Superpod) (metav1.Condition, error) {
	pendingPod := metav1.Condition{
		Status: metav1.ConditionFalse, Reason: "PodNotReady",
		Message: "Waiting for the nginx Pod to be running and pass its readiness probe",
	}

	pod := &corev1.Pod{}
	key := client.ObjectKeyFromObject(resources.NewPod(sp))

	if err := r.Get(ctx, key, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return pendingPod, client.IgnoreNotFound(err)
		}
		return metav1.Condition{}, nil
	}

	if !metav1.IsControlledBy(pod, sp) || !pod.DeletionTimestamp.IsZero() ||
		pod.Status.Phase != corev1.PodRunning {
		return pendingPod, nil
	}

	podReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			podReady = true
		}
	}

	if !podReady {
		return pendingPod, nil
	}

	pendingIngress := metav1.Condition{
		Status: metav1.ConditionFalse, Reason: "IngressPending",
		Message: "Waiting for an Ingress address; check the Ingress controller, class, and address publishing configuration",
	}

	ingress := &networkingv1.Ingress{}

	if err := r.Get(ctx, key, ingress); err != nil {
		return pendingIngress, client.IgnoreNotFound(err)
	}

	if !metav1.IsControlledBy(ingress, sp) || !ingress.DeletionTimestamp.IsZero() {
		return pendingIngress, nil
	}

	for _, address := range ingress.Status.LoadBalancer.Ingress {
		if address.IP != "" || address.Hostname != "" {
			return metav1.Condition{
				Status: metav1.ConditionTrue, Reason: "ResourcesReady",
				Message: "The nginx Pod is ready and the Ingress has an address; browser DNS and connectivity must be configured separately",
			}, nil
		}
	}

	return pendingIngress, nil
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
	condition.Type = "Ready"
	condition.ObservedGeneration = observed.Generation
	// SetStatusCondition preserves LastTransitionTime when the status is unchanged.
	meta.SetStatusCondition(&current.Status.Conditions, condition)
	if equality.Semantic.DeepEqual(before.Status, current.Status) {
		return nil
	}
	
	return r.Status().Update(ctx, current)
}
