package webpage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/webpage"
)

// fakeSelector counts Select calls, standing in for a paid provider.
type fakeSelector struct {
	cacheKey string
	matches  []emoji.Match
	err      error
	calls    int
}

func (f *fakeSelector) CacheKey() string { return f.cacheKey }

func (f *fakeSelector) Select(context.Context, string) ([]emoji.Match, error) {
	f.calls++
	return f.matches, f.err
}

const (
	testCacheKey = "test/"
	flying       = "Flying"
)

var eagle = []emoji.Match{{Emoji: "🦅", Name: "eagle", Score: 1}}

func TestChooseEmojiStoresTheKeyItWasMadeFor(t *testing.T) {
	s := &fakeSelector{cacheKey: testCacheKey, matches: eagle}
	got, err := webpage.ChooseEmoji(context.Background(), s, flying, webpage.Selection{})
	if err != nil {
		t.Fatal(err)
	}
	// sha256(testCacheKey + "\x00" + flying). Existing ConfigMaps store keys computed this way.
	want := webpage.Selection{Key: "4b8bb145034b92e52cf63e5dc6e2a2864dca2d08b417d813204b58bed999f6d6", Matches: eagle}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ChooseEmoji() mismatch (-want +got):\n%s", diff)
	}
	if s.calls != 1 {
		t.Errorf("Select calls = %d, want 1", s.calls)
	}
}

func TestChooseEmojiReusesAValidStoredSelection(t *testing.T) {
	stored, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{cacheKey: testCacheKey, matches: eagle}, flying, webpage.Selection{})
	if err != nil {
		t.Fatal(err)
	}
	// The selector fails, as it does when the operator has no API key; the
	// stored selection must still be served without calling it.
	s := &fakeSelector{cacheKey: testCacheKey, err: errors.New("no API key")}
	got, err := webpage.ChooseEmoji(context.Background(), s, flying, stored)
	if err != nil || s.calls != 0 {
		t.Fatalf("ChooseEmoji() = %v, calls = %d; want the stored selection without calls", err, s.calls)
	}
	if diff := cmp.Diff(stored, got); diff != "" {
		t.Errorf("ChooseEmoji() mismatch (-want +got):\n%s", diff)
	}
}

func TestChooseEmojiSelectsAgainWhenStoredSelectionIsStale(t *testing.T) {
	stored, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{cacheKey: testCacheKey, matches: eagle}, flying, webpage.Selection{})
	if err != nil {
		t.Fatal(err)
	}
	tampered := webpage.Selection{Key: stored.Key, Matches: []emoji.Match{{Emoji: "🦅", Name: "not the catalog name", Score: 1}}}
	for _, test := range []struct {
		name     string
		cacheKey string
		ability  string
		stored   webpage.Selection
	}{
		{"ability changed", testCacheKey, "Invisibility", stored},
		{"selector changed", testCacheKey + "other-model", flying, stored},
		{"stored matches invalid", testCacheKey, flying, tampered},
		{"nothing stored", testCacheKey, flying, webpage.Selection{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := &fakeSelector{cacheKey: test.cacheKey, matches: eagle}
			if _, err := webpage.ChooseEmoji(context.Background(), s, test.ability, test.stored); err != nil {
				t.Fatal(err)
			}
			if s.calls != 1 {
				t.Errorf("Select calls = %d, want 1", s.calls)
			}
		})
	}
}

func TestChooseEmojiErrors(t *testing.T) {
	if _, err := webpage.ChooseEmoji(context.Background(), nil, flying, webpage.Selection{}); !errors.Is(err, webpage.ErrNoEmojiSelector) {
		t.Errorf("nil selector: err = %v, want ErrNoEmojiSelector", err)
	}
	unavailable := errors.New("unavailable")
	if _, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{err: unavailable}, flying, webpage.Selection{}); !errors.Is(err, unavailable) {
		t.Errorf("failing selector: err = %v, want %v", err, unavailable)
	}
	for _, matches := range [][]emoji.Match{nil, {{Emoji: "not an emoji", Name: "x", Score: 1}}} {
		if _, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{matches: matches}, flying, webpage.Selection{}); err == nil {
			t.Errorf("accepted invalid matches %v", matches)
		}
	}
}

func TestSelectionSymbols(t *testing.T) {
	sel := webpage.Selection{Matches: []emoji.Match{{Emoji: "🦅"}, {Emoji: "🪽"}}}
	if diff := cmp.Diff([]string{"🦅", "🪽"}, sel.Symbols()); diff != "" {
		t.Errorf("Symbols() mismatch (-want +got):\n%s", diff)
	}
}
