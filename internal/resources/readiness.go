package resources

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// Readiness returns sp's Ready condition from its observed Pod and Ingress; nil
// means the object does not exist. It judges Kubernetes signals only: the Pod's
// readiness probe checks nginx and the Ingress controller publishes an address.
// Browser DNS and connectivity are not verified.
func Readiness(sp *superv1.Superpod, pod *corev1.Pod, ingress *networkingv1.Ingress) metav1.Condition {
	if pod == nil || !controlledAndLive(sp, pod) || pod.Status.Phase != corev1.PodRunning || !podReady(pod) {
		return metav1.Condition{
			Type: superv1.ReadyCondition, Status: metav1.ConditionFalse, Reason: superv1.PodNotReadyReason,
			Message: "Waiting for the nginx Pod to be running and pass its readiness probe",
		}
	}
	if ingress == nil || !controlledAndLive(sp, ingress) || !hasAddress(ingress) {
		return metav1.Condition{
			Type: superv1.ReadyCondition, Status: metav1.ConditionFalse, Reason: superv1.IngressPendingReason,
			Message: "Waiting for an Ingress address; check the Ingress controller, class, and address publishing configuration",
		}
	}
	return metav1.Condition{
		Type: superv1.ReadyCondition, Status: metav1.ConditionTrue, Reason: superv1.ResourcesReadyReason,
		Message: "The nginx Pod is ready and the Ingress has an address; browser DNS and connectivity must be configured separately",
	}
}

// controlledAndLive reports whether obj belongs to sp and is not being deleted.
func controlledAndLive(sp *superv1.Superpod, obj metav1.Object) bool {
	return metav1.IsControlledBy(obj, sp) && obj.GetDeletionTimestamp().IsZero()
}

func podReady(pod *corev1.Pod) bool {
	return slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue
	})
}

func hasAddress(ingress *networkingv1.Ingress) bool {
	return slices.ContainsFunc(ingress.Status.LoadBalancer.Ingress, func(a networkingv1.IngressLoadBalancerIngress) bool {
		return a.IP != "" || a.Hostname != ""
	})
}
