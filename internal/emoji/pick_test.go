package emoji

import (
	"testing"
	"time"
)

func TestPick(t *testing.T) {
	symbols := []string{"🦅", "🪽", "✈️", "🚀"}
	ranked := func(scores ...float64) []Match {
		matches := make([]Match, len(scores))
		for i, score := range scores {
			matches[i] = Match{Emoji: symbols[i], Name: catalog[symbols[i]], Score: score}
		}
		return matches
	}
	for _, test := range []struct {
		name   string
		ranked []Match
		want   int
	}{
		{"nothing ranked", nil, 0},
		{"clear winner", ranked(.8, .1, .06, .04), 1},
		{"two close", ranked(.4, .35, .2, .05), 2},
		{"four close", ranked(.26, .25, .25, .24), 4},
		{"three close fill the square", ranked(.3, .28, .27, .15), 4},
		{"three close without a fourth", ranked(.3, .28, .27), 2},
		{"compare to winner not neighbor", ranked(.36, .32, .28, .04), 2},
		{"gap equal to the threshold", ranked(.5, .45), 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Pick(test.ranked)
			if len(got) != test.want {
				t.Fatalf("Pick() kept %d matches, want %d: %+v", len(got), test.want, got)
			}
			if test.want > 0 && !ValidMatches(got) {
				t.Fatalf("Pick() returned invalid matches: %+v", got)
			}
		})
	}
}

func TestCatalogIsACopy(t *testing.T) {
	c := Catalog()
	if len(c) < 3900 || c["🦅"] != "eagle" || c["👩🏽‍🚀"] == "" || c["🇲🇽"] == "" {
		t.Fatal("catalog must include Unicode sequences, modifiers, and flags")
	}
	delete(c, "🦅")
	if Catalog()["🦅"] != "eagle" {
		t.Fatal("callers must not be able to change the catalog")
	}
	if v := CatalogVersion(); len(v) != 64 || v != CatalogVersion() {
		t.Fatalf("CatalogVersion() = %q, want a stable SHA-256 hex digest", v)
	}
}

func TestRateLimitErrorMessage(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	err := &RateLimitError{RetryAt: at, Cause: "Jev returned HTTP 429 (rate limit exceeded)"}
	want := "Jev returned HTTP 429 (rate limit exceeded); next attempt after 2026-09-28T12:00:00Z"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	if got := (&RateLimitError{RetryAt: at}).Error(); got != "emoji provider rate limit exceeded; next attempt after 2026-09-28T12:00:00Z" {
		t.Errorf("Error() without a cause = %q", got)
	}
}
