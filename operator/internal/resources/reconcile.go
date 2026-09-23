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
	"context"
	"fmt"
	"maps"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// Ensure creates a missing child or updates the fields this operator manages.
// It returns true when an existing child is still being deleted.
func Ensure(ctx context.Context, c client.Client, scheme *runtime.Scheme, sp *superv1.Superpod, desired client.Object) (bool, error) {
	if err := controllerutil.SetControllerReference(sp, desired, scheme); err != nil {
		return false, err
	}
	// Get into an empty object of the same kind. Starting with desired values
	// could hide missing fields when the API response is decoded.
	current := reflect.New(reflect.TypeOf(desired).Elem()).Interface().(client.Object)
	key := client.ObjectKeyFromObject(desired)
	if err := c.Get(ctx, key, current); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		if err := c.Create(ctx, desired); err != nil {
			// An AlreadyExists race is safe: returning the error retries the
			// reconciliation, which will fetch and check ownership next time.
			return false, err
		}
		log.FromContext(ctx).Info("Created child resource", "kind", fmt.Sprintf("%T", desired), "name", key.Name)
		return false, nil
	}
	if !metav1.IsControlledBy(current, sp) {
		return false, fmt.Errorf("refusing to modify %T %s: it is not controlled by Superpod %s", current, key, sp.Name)
	}
	if !current.GetDeletionTimestamp().IsZero() {
		return true, nil
	}

	before := current.DeepCopyObject()
	// Preserve labels and annotations belonging to other tools.
	labels := current.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	maps.Copy(labels, desired.GetLabels())
	current.SetLabels(labels)
	if err := controllerutil.SetControllerReference(sp, current, scheme); err != nil {
		return false, err
	}
	applyManagedFields(current, desired)
	if equality.Semantic.DeepEqual(before, current) {
		return false, nil
	}
	// Update uses the resourceVersion from Get; conflicts are retried by the
	// controller. Never overwrite Kubernetes-assigned fields with empty values.
	if err := c.Update(ctx, current); err != nil {
		return false, err
	}
	log.FromContext(ctx).Info("Updated child resource", "kind", fmt.Sprintf("%T", current), "name", key.Name)
	return false, nil
}

func applyManagedFields(current, desired client.Object) {
	switch obj := current.(type) {
	case *corev1.ConfigMap:
		obj.Data = desired.(*corev1.ConfigMap).Data
	case *corev1.ServiceAccount:
		obj.AutomountServiceAccountToken = desired.(*corev1.ServiceAccount).AutomountServiceAccountToken
	case *corev1.Pod:
		// Most Pod fields are immutable. Keep the existing spec and defaults;
		// HTML updates only change the ConfigMap. Container images are mutable.
		for i := range obj.Spec.Containers {
			for _, container := range desired.(*corev1.Pod).Spec.Containers {
				if obj.Spec.Containers[i].Name == container.Name {
					obj.Spec.Containers[i].Image = container.Image
				}
			}
		}
	case *corev1.Service:
		wanted := desired.(*corev1.Service)
		obj.Spec.Type = wanted.Spec.Type
		obj.Spec.Selector = wanted.Spec.Selector
		obj.Spec.Ports = wanted.Spec.Ports
		// Keep ClusterIP, IPFamilies, and other fields allocated by Kubernetes.
	case *networkingv1.Ingress:
		wanted := desired.(*networkingv1.Ingress)
		obj.Spec.Rules = wanted.Spec.Rules
		if wanted.Spec.IngressClassName != nil {
			obj.Spec.IngressClassName = wanted.Spec.IngressClassName
		}
		// If no class is requested, retain the class assigned at creation.
		// Clearing an admission-defaulted class would cause repeated updates.
	}
}
