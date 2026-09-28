package resources

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/webpage"
)

// ConfigMap keys. nginx serves pageKey; selectionKey keeps the emoji selection
// behind the page so later reconciles can rebuild it without a provider call.
const (
	pageKey      = "index.html"
	selectionKey = "emoji-cache.json"
)

// NewConfigMap builds the ConfigMap that nginx serves. When sel is non-nil, its
// emoji are shown on the page and sel is stored for later reconciles.
func NewConfigMap(sp *superv1.Superpod, sel *webpage.Selection) (*corev1.ConfigMap, error) {
	content := webpage.Content{Title: sp.Name, Ability: sp.Spec.SuperAbility}
	if sel != nil {
		content.Emoji = sel.Symbols()
	}
	page, err := webpage.Render(content)
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metadata(sp),
		Data:       map[string]string{pageKey: page},
	}
	if sel != nil {
		data, err := json.Marshal(sel)
		if err != nil {
			return nil, fmt.Errorf("encode emoji selection: %w", err)
		}
		cm.Data[selectionKey] = string(data)
	}
	return cm, nil
}

// StoredSelection returns the emoji selection kept in cm. It returns the zero
// Selection when cm is nil or holds no readable selection, which makes
// webpage.ChooseEmoji select again.
func StoredSelection(cm *corev1.ConfigMap) webpage.Selection {
	var sel webpage.Selection
	if cm == nil || json.Unmarshal([]byte(cm.Data[selectionKey]), &sel) != nil {
		return webpage.Selection{}
	}
	return sel
}
