package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
	"github.com/jairjosafath/operator/internal/webpage"
)

// chooseEmoji returns the emoji selection for sp's webpage, or nil for a plain
// page. The selection stored in the current ConfigMap is reused when still
// valid, because each new selection is a paid provider call.
func (r *SuperpodReconciler) chooseEmoji(ctx context.Context, sp *superv1.Superpod) (*webpage.Selection, error) {
	if !sp.Spec.Emoji {
		return nil, nil
	}
	current, err := getIfExists(ctx, r, resources.ObjectKey(sp), &corev1.ConfigMap{})
	if err != nil {
		return nil, err
	}
	if current != nil && (!metav1.IsControlledBy(current, sp) || !current.DeletionTimestamp.IsZero()) {
		// Ensure reports the name collision or waits for deletion. Do not pay
		// for a selection that cannot be stored yet.
		return nil, nil
	}
	sel, err := webpage.ChooseEmoji(ctx, r.EmojiSelector, sp.Spec.SuperAbility, resources.StoredSelection(current))
	if err != nil {
		return nil, err
	}
	return &sel, nil
}
