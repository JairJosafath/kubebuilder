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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/resources"
)

const invisibilityAbility = "Invisibility"

var _ = Describe("Superpod reconciliation", func() {
	var sp *superv1.Superpod
	var reconciler *SuperpodReconciler
	var request ctrl.Request

	// Envtest runs an API server but no garbage collector or kubelet. These
	// tests check ownership; cleanup is explicit instead of waiting for GC.
	childObjects := func() []client.Object {
		return []client.Object{
			&corev1.ServiceAccount{}, &corev1.ConfigMap{}, &corev1.Pod{},
			&corev1.Service{}, &networkingv1.Ingress{},
		}
	}
	fetchChildren := func() []client.Object {
		children := childObjects()
		for _, child := range children {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resources.NewPod(sp)), child)).To(Succeed())
		}
		return children
	}
	reconcileOnce := func() {
		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))
	}

	BeforeEach(func() {
		sp = &superv1.Superpod{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "reconcile-test-", Namespace: "default"},
			Spec:       superv1.SuperpodSpec{SuperAbility: "Flying", Host: "superpod.example.test"},
		}
		Expect(k8sClient.Create(ctx, sp)).To(Succeed())
		request = ctrl.Request{NamespacedName: client.ObjectKeyFromObject(sp)}
		reconciler = &SuperpodReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	})

	AfterEach(func() {
		objects := append(childObjects(), &superv1.Superpod{})
		for _, obj := range objects {
			key := client.ObjectKeyFromObject(resources.NewPod(sp))
			if _, isSuperpod := obj.(*superv1.Superpod); isSuperpod {
				key = request.NamespacedName
			}
			err := k8sClient.Get(ctx, key, obj)
			if apierrors.IsNotFound(err) {
				continue
			}
			Expect(err).NotTo(HaveOccurred())
			if len(obj.GetFinalizers()) > 0 {
				obj.SetFinalizers(nil)
				Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj, client.GracePeriodSeconds(0)))).To(Succeed())
		}
	})

	It("opts into emoji, caches across reconciler restarts, and refreshes only when necessary", func() {
		selector := &testEmojiSelector{}
		reconciler.EmojiSelector = selector
		reconcileOnce()
		Expect(selector.calls).To(BeZero())
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		sp.Spec.Emoji = true
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		reconcileOnce()
		Expect(selector.calls).To(Equal(1))
		cm := &corev1.ConfigMap{}
		key := client.ObjectKeyFromObject(resources.NewConfigMap(sp))
		Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
		Expect(cm.Data["index.html"]).To(ContainSubstring("🦅"))
		version := cm.ResourceVersion
		pod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, key, pod)).To(Succeed())
		podUID := pod.UID
		reconciler = &SuperpodReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), EmojiSelector: selector}
		reconcileOnce()
		Expect(selector.calls).To(Equal(1))
		Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
		Expect(cm.ResourceVersion).To(Equal(version))

		// Repair HTML tampering from the cached selection without another call.
		cm.Data["index.html"] = "changed"
		Expect(k8sClient.Update(ctx, cm)).To(Succeed())
		reconcileOnce()
		Expect(selector.calls).To(Equal(1))
		Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
		Expect(cm.Data["index.html"]).To(ContainSubstring("🦅"))

		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		sp.Spec.SuperAbility = invisibilityAbility
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		reconcileOnce()
		Expect(selector.calls).To(Equal(2))
		Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
		Expect(cm.Data["index.html"]).To(ContainSubstring("👻"))
		Expect(cm.Data["index.html"]).NotTo(ContainSubstring("🦅"))
		Expect(k8sClient.Get(ctx, key, pod)).To(Succeed())
		Expect(pod.UID).To(Equal(podUID))

		selector.version = "changed-model"
		reconcileOnce()
		Expect(selector.calls).To(Equal(3))
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		sp.Spec.Emoji = false
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		reconcileOnce()
		Expect(selector.calls).To(Equal(3))
		Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
		Expect(cm.Data).To(Equal(resources.NewConfigMap(sp).Data))
	})

	It("serves plain HTML on Jev failure and recovers on retry", func() {
		sp.Spec.Emoji = true
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		selector := &testEmojiSelector{err: errors.New("Jev returned HTTP 429")}
		reconciler.EmojiSelector = selector
		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Minute))
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(meta.FindStatusCondition(sp.Status.Conditions, "Ready").Reason).To(Equal("EmojiSelectionFailed"))
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resources.NewConfigMap(sp)), cm)).To(Succeed())
		Expect(cm.Data).To(Equal(resources.NewConfigMap(sp).Data))
		Expect(fetchChildren()).To(HaveLen(5))
		selector.err = nil
		reconcileOnce()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(meta.FindStatusCondition(sp.Status.Conditions, "Ready").Reason).To(Equal("PodNotReady"))
	})

	It("creates owned children and makes no writes on repeated reconciles", func() {
		reconcileOnce()
		before := fetchChildren()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(sp.Status.PodName).To(Equal(resources.NewPod(sp).Name))
		Expect(sp.Status.URL).To(Equal("http://" + sp.Spec.Host))
		ready := meta.FindStatusCondition(sp.Status.Conditions, "Ready")
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal("PodNotReady"))
		Expect(ready.ObservedGeneration).To(Equal(sp.Generation))
		parentVersion := sp.ResourceVersion
		for _, child := range before {
			owner := metav1.GetControllerOf(child)
			Expect(owner).NotTo(BeNil())
			Expect(owner.UID).To(Equal(sp.UID))
			Expect(owner.BlockOwnerDeletion).To(HaveValue(BeTrue()))
		}
		service := before[3].(*corev1.Service)
		Expect(service.Spec.ClusterIP).NotTo(BeEmpty())
		for range 3 {
			reconcileOnce()
		}
		after := fetchChildren()
		for i := range before {
			Expect(after[i].GetUID()).To(Equal(before[i].GetUID()))
			Expect(after[i].GetResourceVersion()).To(Equal(before[i].GetResourceVersion()))
		}
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(sp.ResourceVersion).To(Equal(parentVersion))
		pods := &corev1.PodList{}
		Expect(k8sClient.List(ctx, pods, client.InNamespace(sp.Namespace),
			client.MatchingLabels(resources.NewService(sp).Spec.Selector))).To(Succeed())
		Expect(pods.Items).To(HaveLen(1))
	})

	It("repairs managed fields without replacing the Pod or Service IP", func() {
		reconcileOnce()
		before := fetchChildren()
		cm := before[1].(*corev1.ConfigMap)
		cm.Data = nil
		cm.Labels = nil
		cm.Annotations = map[string]string{"example.test/note": "keep"}
		Expect(k8sClient.Update(ctx, cm)).To(Succeed())
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		sp.Spec.SuperAbility = invisibilityAbility
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())

		reconcileOnce()
		after := fetchChildren()
		updated := after[1].(*corev1.ConfigMap)
		Expect(updated.Data["index.html"]).To(ContainSubstring(invisibilityAbility))
		Expect(updated.Labels).To(Equal(resources.NewConfigMap(sp).Labels))
		Expect(updated.Annotations).To(HaveKeyWithValue("example.test/note", "keep"))
		Expect(after[2].GetUID()).To(Equal(before[2].GetUID()))
		Expect(after[2].GetResourceVersion()).To(Equal(before[2].GetResourceVersion()))
		Expect(after[3].(*corev1.Service).Spec.ClusterIP).To(Equal(before[3].(*corev1.Service).Spec.ClusterIP))
	})

	It("recreates any missing child with the same name and owner", func() {
		reconcileOnce()
		for _, child := range fetchChildren() {
			oldUID := child.GetUID()
			Expect(k8sClient.Delete(ctx, child, client.GracePeriodSeconds(0))).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(child), child))
			}).WithTimeout(5 * time.Second).Should(BeTrue())
			reconcileOnce()
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(child), child)).To(Succeed())
			Expect(child.GetUID()).NotTo(Equal(oldUID))
			Expect(metav1.IsControlledBy(child, sp)).To(BeTrue())
		}
	})

	DescribeTable("refuses to adopt a ConfigMap belonging to someone else", func(foreignOwner bool) {
		cm := resources.NewConfigMap(sp)
		cm.Data = map[string]string{"index.html": "unrelated"}
		if foreignOwner {
			other := sp.DeepCopy()
			other.UID = "6f09dddb-d1bb-4494-8573-520e49150697"
			other.Name = "another-superpod"
			Expect(controllerutil.SetControllerReference(other, cm, k8sClient.Scheme())).To(Succeed())
		}
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())
		version := cm.ResourceVersion
		_, err := reconciler.Reconcile(ctx, request)
		Expect(err).To(MatchError(ContainSubstring("refusing to modify")))
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.ResourceVersion).To(Equal(version))
		Expect(cm.Data["index.html"]).To(Equal("unrelated"))
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(meta.FindStatusCondition(sp.Status.Conditions, "Ready").Reason).To(Equal("ReconcileFailed"))
	}, Entry("unowned resource", false), Entry("another owner", true))

	It("waits for a terminating child before recreating it", func() {
		reconcileOnce()
		cm := &corev1.ConfigMap{}
		key := client.ObjectKeyFromObject(resources.NewConfigMap(sp))
		Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
		cm.Finalizers = []string{"example.test/hold"}
		Expect(k8sClient.Update(ctx, cm)).To(Succeed())
		Expect(k8sClient.Delete(ctx, cm)).To(Succeed())
		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(meta.FindStatusCondition(sp.Status.Conditions, "Ready").Reason).To(Equal("ResourceTerminating"))
		Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
		Expect(cm.DeletionTimestamp.IsZero()).To(BeFalse())
		cm.Finalizers = nil
		Expect(k8sClient.Update(ctx, cm)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, key, cm))
		}).WithTimeout(5 * time.Second).Should(BeTrue())
		reconcileOnce()
	})

	It("does not recreate children for a deleting or missing Superpod", func() {
		sp.Finalizers = []string{"example.test/hold"}
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		Expect(k8sClient.Delete(ctx, sp)).To(Succeed())
		reconcileOnce()
		for _, child := range childObjects() {
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx,
				client.ObjectKeyFromObject(resources.NewPod(sp)), child))).To(BeTrue())
		}
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		sp.Finalizers = nil
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, request.NamespacedName, &superv1.Superpod{}))
		}).WithTimeout(5 * time.Second).Should(BeTrue())
		reconcileOnce()
	})

	It("uses watches to recreate a Pod and report readiness without manual reconciliation", func() {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:                 k8sClient.Scheme(),
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())
		watching := &SuperpodReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
		Expect(watching.SetupWithManager(mgr)).To(Succeed())
		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		defer func() {
			stop()
			Eventually(done).WithTimeout(10 * time.Second).Should(Receive(BeNil()))
		}()

		pod := &corev1.Pod{}
		key := client.ObjectKeyFromObject(resources.NewPod(sp))
		Eventually(func() error {
			return k8sClient.Get(ctx, key, pod)
		}).WithTimeout(10 * time.Second).Should(Succeed())
		oldUID := pod.UID
		Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed())
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, key, pod); err != nil {
				return false
			}
			return pod.UID != oldUID && metav1.IsControlledBy(pod, sp)
		}).WithTimeout(10 * time.Second).Should(BeTrue())

		// There is no kubelet or Ingress controller in envtest. Simulate their
		// status updates and check that our watches trigger status reporting.
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		Eventually(func() string {
			if err := k8sClient.Get(ctx, request.NamespacedName, sp); err != nil {
				return ""
			}
			if condition := meta.FindStatusCondition(sp.Status.Conditions, "Ready"); condition != nil {
				return condition.Reason
			}
			return ""
		}).WithTimeout(10 * time.Second).Should(Equal("IngressPending"))
		ingress := &networkingv1.Ingress{}
		Expect(k8sClient.Get(ctx, key, ingress)).To(Succeed())
		ingress.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: "192.0.2.10"}}
		Expect(k8sClient.Status().Update(ctx, ingress)).To(Succeed())
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, request.NamespacedName, sp); err != nil {
				return false
			}
			return meta.IsStatusConditionTrue(sp.Status.Conditions, "Ready")
		}).WithTimeout(10 * time.Second).Should(BeTrue())
	})

	It("updates the URL and generation, and clears readiness when the Pod becomes unready", func() {
		reconcileOnce()
		pod := &corev1.Pod{}
		key := client.ObjectKeyFromObject(resources.NewPod(sp))
		Expect(k8sClient.Get(ctx, key, pod)).To(Succeed())
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		ingress := &networkingv1.Ingress{}
		Expect(k8sClient.Get(ctx, key, ingress)).To(Succeed())
		ingress.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{Hostname: "ingress.example.test"}}
		Expect(k8sClient.Status().Update(ctx, ingress)).To(Succeed())
		reconcileOnce()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(meta.IsStatusConditionTrue(sp.Status.Conditions, "Ready")).To(BeTrue())
		transition := meta.FindStatusCondition(sp.Status.Conditions, "Ready").LastTransitionTime
		version := sp.ResourceVersion
		reconcileOnce()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(sp.ResourceVersion).To(Equal(version))

		sp.Spec.Host = "updated.example.test"
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		reconcileOnce()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(sp.Status.URL).To(Equal("http://updated.example.test"))
		condition := meta.FindStatusCondition(sp.Status.Conditions, "Ready")
		Expect(condition.ObservedGeneration).To(Equal(sp.Generation))
		Expect(condition.LastTransitionTime).To(Equal(transition))
		Expect(k8sClient.Get(ctx, key, ingress)).To(Succeed())
		Expect(ingress.Spec.Rules[0].Host).To(Equal(sp.Spec.Host))

		pod.Status.Conditions[0].Status = corev1.ConditionFalse
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		reconcileOnce()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(meta.IsStatusConditionFalse(sp.Status.Conditions, "Ready")).To(BeTrue())
	})

	It("does not publish a status computed for an older spec", func() {
		reconcileOnce()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		stale := sp.DeepCopy()
		sp.Spec.Host = "newer.example.test"
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		err := reconciler.updateStatus(ctx, stale, resources.NewPod(stale).Name, metav1.Condition{
			Status: metav1.ConditionTrue, Reason: "ResourcesReady", Message: "Stale report",
		})
		Expect(apierrors.IsConflict(err)).To(BeTrue())
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(sp.Status).To(Equal(stale.Status))
		reconcileOnce()
		Expect(k8sClient.Get(ctx, request.NamespacedName, sp)).To(Succeed())
		Expect(sp.Status.URL).To(Equal("http://newer.example.test"))
	})
})

type testEmojiSelector struct {
	calls   int
	err     error
	version string
}

func (s *testEmojiSelector) CacheKey() string { return "test/" + s.version }

func (s *testEmojiSelector) Select(_ context.Context, ability string) ([]emoji.Match, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if ability == invisibilityAbility {
		return []emoji.Match{{Emoji: "👻", Name: "ghost", Score: 1}}, nil
	}
	return []emoji.Match{{Emoji: "🦅", Name: "eagle", Score: 1}}, nil
}
