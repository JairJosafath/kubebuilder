package resources_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
)

func TestReadiness(t *testing.T) {
	sp := exampleSuperpod()
	owner := []metav1.OwnerReference{*metav1.NewControllerRef(sp, superv1.GroupVersion.WithKind("Superpod"))}
	deleting := metav1.Now()

	// pod and ingress return ready, owned objects; each case changes one thing.
	pod := func(change func(*corev1.Pod)) *corev1.Pod {
		p := resources.NewPod(sp)
		p.OwnerReferences = owner
		p.Status.Phase = corev1.PodRunning
		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		change(p)
		return p
	}
	ingress := func(change func(*networkingv1.Ingress)) *networkingv1.Ingress {
		i := resources.NewIngress(sp)
		i.OwnerReferences = owner
		i.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: "192.0.2.10"}}
		change(i)
		return i
	}

	for _, test := range []struct {
		name    string
		pod     *corev1.Pod
		ingress *networkingv1.Ingress
		status  metav1.ConditionStatus
		reason  string
	}{
		{"ready with IP", pod(func(*corev1.Pod) {}), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionTrue, superv1.ResourcesReadyReason},
		{"ready with hostname", pod(func(*corev1.Pod) {}), ingress(func(i *networkingv1.Ingress) {
			i.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{Hostname: "ingress.example.test"}}
		}), metav1.ConditionTrue, superv1.ResourcesReadyReason},
		{"Pod missing", nil, ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod owned by someone else", pod(func(p *corev1.Pod) { p.OwnerReferences = nil }), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod deleting", pod(func(p *corev1.Pod) { p.DeletionTimestamp = &deleting }), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod pending", pod(func(p *corev1.Pod) { p.Status.Phase = corev1.PodPending }), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod failing its probe", pod(func(p *corev1.Pod) { p.Status.Conditions[0].Status = corev1.ConditionFalse }),
			ingress(func(*networkingv1.Ingress) {}), metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Ingress missing", pod(func(*corev1.Pod) {}), nil,
			metav1.ConditionFalse, superv1.IngressPendingReason},
		{"Ingress owned by someone else", pod(func(*corev1.Pod) {}), ingress(func(i *networkingv1.Ingress) { i.OwnerReferences = nil }),
			metav1.ConditionFalse, superv1.IngressPendingReason},
		{"Ingress without address", pod(func(*corev1.Pod) {}), ingress(func(i *networkingv1.Ingress) {
			i.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{}}
		}), metav1.ConditionFalse, superv1.IngressPendingReason},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := resources.Readiness(sp, test.pod, test.ingress)
			if got.Type != superv1.ReadyCondition || got.Status != test.status || got.Reason != test.reason || got.Message == "" {
				t.Errorf("Readiness() = %s/%s/%s, want %s/%s/%s with a message",
					got.Type, got.Status, got.Reason, superv1.ReadyCondition, test.status, test.reason)
			}
		})
	}
}
