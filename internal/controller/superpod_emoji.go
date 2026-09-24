package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/resources"
)

// EmojiSelector isolates the external service from Kubernetes reconciliation.
type EmojiSelector interface {
	Select(context.Context, string) ([]emoji.Match, error)
	CacheKey() string
}

type emojiCache struct {
	Key     string        `json:"key"`
	Matches []emoji.Match `json:"matches"`
}

const emojiCacheFile = "emoji-cache.json"

// emojiPage persists a successful selection with the HTML. Pod, status, and
// Ingress events can then rebuild the page without repeating paid API calls.
func (r *SuperpodReconciler) emojiPage(ctx context.Context, sp *superv1.Superpod) (*corev1.ConfigMap, error) {
	page := resources.NewConfigMap(sp)
	if !sp.Spec.Emoji {
		return page, nil
	}
	if r.EmojiSelector == nil {
		return page, errors.New("emoji matching requires JEV_API_KEY in the operator environment")
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(r.EmojiSelector.CacheKey()+"\x00"+sp.Spec.SuperAbility)))
	current := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(page), current)
	if err != nil && !apierrors.IsNotFound(err) {
		return page, err
	}
	if err == nil && (!metav1.IsControlledBy(current, sp) || !current.DeletionTimestamp.IsZero()) {
		// Let Ensure report collisions or wait for deletion without spending an API call.
		return page, nil
	}
	var cached emojiCache
	if json.Unmarshal([]byte(current.Data[emojiCacheFile]), &cached) != nil ||
		cached.Key != key || !emoji.ValidMatches(cached.Matches) {
		matches, selectErr := r.EmojiSelector.Select(ctx, sp.Spec.SuperAbility)
		if selectErr != nil {
			return page, selectErr
		}
		if !emoji.ValidMatches(matches) {
			return page, errors.New("emoji selector returned invalid matches")
		}
		cached = emojiCache{Key: key, Matches: matches}
	}
	symbols := make([]string, 0, len(cached.Matches))
	for _, match := range cached.Matches {
		symbols = append(symbols, match.Emoji)
	}
	page = resources.NewConfigMap(sp, symbols...)
	data, err := json.Marshal(cached)
	if err != nil {
		return page, err
	}
	page.Data[emojiCacheFile] = string(data)
	return page, nil
}
