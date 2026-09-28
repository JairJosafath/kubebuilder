package webpage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/jairjosafath/operator/internal/emoji"
)

// EmojiSelector chooses the emoji shown for an ability. Implementations usually
// call a paid external service, which is why ChooseEmoji reuses selections.
type EmojiSelector interface {
	// Select returns the emoji to show for ability, best first.
	Select(ctx context.Context, ability string) ([]emoji.Match, error)
	// CacheKey identifies everything besides the ability that affects what
	// Select returns, such as the provider, model, and catalog.
	CacheKey() string
}

// Selection is an emoji choice and the key it was made for. It is stored next
// to the webpage so later reconciles can reuse it.
type Selection struct {
	Key     string        `json:"key"`
	Matches []emoji.Match `json:"matches"`
}

// Symbols returns the selected emoji in display order.
func (s Selection) Symbols() []string {
	symbols := make([]string, 0, len(s.Matches))
	for _, m := range s.Matches {
		symbols = append(symbols, m.Emoji)
	}
	return symbols
}

// ErrNoEmojiSelector reports that emoji were requested but no selector is configured.
var ErrNoEmojiSelector = errors.New("no emoji selector is configured")

// ChooseEmoji returns stored when it is valid and was made for ability by an
// equivalent selector. Otherwise it asks s for a new selection. Each Superpod
// therefore pays for one selection per ability and selector configuration.
func ChooseEmoji(ctx context.Context, s EmojiSelector, ability string, stored Selection) (Selection, error) {
	if s == nil {
		return Selection{}, ErrNoEmojiSelector
	}
	key := selectionKey(s.CacheKey(), ability)
	if stored.Key == key && emoji.ValidMatches(stored.Matches) {
		return stored, nil
	}
	matches, err := s.Select(ctx, ability)
	if err != nil {
		return Selection{}, err
	}
	if !emoji.ValidMatches(matches) {
		return Selection{}, errors.New("emoji selector returned invalid matches")
	}
	return Selection{Key: key, Matches: matches}, nil
}

// selectionKey is persisted in every Superpod's ConfigMap. Changing how it is
// computed makes each Superpod pay for a new selection.
func selectionKey(cacheKey, ability string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(cacheKey+"\x00"+ability)))
}
