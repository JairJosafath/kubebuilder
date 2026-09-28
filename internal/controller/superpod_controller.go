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
	"github.com/jairjosafath/operator/internal/webpage"
)

// SuperpodReconciler reconciles a Superpod object.
type SuperpodReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EmojiSelector webpage.EmojiSelector
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

	// A failed emoji selection must not take the webpage down: the plain page
	// is applied first and the failure is reported afterwards.
	sel, emojiErr := r.chooseEmoji(ctx, sp)
	page, err := resources.NewConfigMap(sp, sel)
	if err != nil {
		return r.fail(ctx, sp, err)
	}
	pod := resources.NewPod(sp)
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
			return r.fail(ctx, sp, err)
		}
		if terminating {
			// A name cannot be reused until deletion finishes. Watches normally
			// wake us up; this short retry also covers a missed deletion event.
			return ctrl.Result{RequeueAfter: time.Second}, r.updateStatus(ctx, sp, sp.Status.PodName, metav1.Condition{
				Status: metav1.ConditionFalse, Reason: superv1.ResourceTerminatingReason,
				Message: "Waiting for a child resource to finish deletion before recreating it",
			})
		}
	}

	if emojiErr != nil {
		delay, reason := time.Minute, superv1.EmojiSelectionFailedReason
		if rateLimit, ok := errors.AsType[*emoji.RateLimitError](emojiErr); ok {
			delay, reason = max(time.Second, time.Until(rateLimit.RetryAt)), superv1.EmojiRateLimitedReason
		}
		return ctrl.Result{RequeueAfter: delay}, r.updateStatus(ctx, sp, pod.Name, metav1.Condition{
			Status: metav1.ConditionFalse, Reason: reason, Message: emojiErr.Error(),
		})
	}

	condition, err := r.readiness(ctx, sp)
	if err != nil {
		statusErr := r.updateStatus(ctx, sp, pod.Name, metav1.Condition{
			Status: metav1.ConditionUnknown, Reason: superv1.ObservationFailedReason, Message: err.Error(),
		})
		return ctrl.Result{}, errors.Join(err, statusErr)
	}
	return ctrl.Result{}, r.updateStatus(ctx, sp, pod.Name, condition)
}

// fail reports err in the Ready condition and returns it, so controller-runtime
// retries the request with backoff.
func (r *SuperpodReconciler) fail(ctx context.Context, sp *superv1.Superpod, err error) (ctrl.Result, error) {
	statusErr := r.updateStatus(ctx, sp, sp.Status.PodName, metav1.Condition{
		Status: metav1.ConditionFalse, Reason: superv1.ReconcileFailedReason, Message: err.Error(),
	})
	return ctrl.Result{}, errors.Join(err, statusErr)
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

// getIfExists reads obj by key. It returns nil, without an error, when the
// object does not exist.
func getIfExists[T client.Object](ctx context.Context, c client.Reader, key client.ObjectKey, obj T) (T, error) {
	if err := c.Get(ctx, key, obj); err != nil {
		var none T
		return none, client.IgnoreNotFound(err)
	}
	return obj, nil
}
