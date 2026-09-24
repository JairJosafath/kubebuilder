package emoji

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryDeadline(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		header string
		want   time.Duration
	}{
		{"120", 2 * time.Minute},
		{now.Add(time.Hour).Format(http.TimeFormat), time.Hour},
		{"", time.Minute}, {"invalid", time.Minute}, {"-1", time.Minute},
		{"0", time.Minute}, {"9223372036854775807", time.Minute},
		{now.Add(-time.Hour).Format(http.TimeFormat), time.Minute},
	} {
		t.Run(test.header, func(t *testing.T) {
			if got := retryDeadline(test.header, now, time.Minute); !got.Equal(now.Add(test.want)) {
				t.Fatalf("retry at %s, want %s", got, now.Add(test.want))
			}
		})
	}
}

func TestCooldownPreventsRequestsAcrossAbilities(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	now := time.Now()
	c := NewClient(testAPIKey, "")
	c.endpoint, c.now = server.URL, func() time.Time { return now }
	for _, ability := range []string{flyingAbility, flyingAbility, "Invisibility"} {
		_, err := c.Select(context.Background(), ability)
		var limited *RateLimitError
		if !errors.As(err, &limited) || !limited.RetryAt.Equal(now.Add(2*time.Minute)) {
			t.Fatalf("expected shared cooldown, got %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("early reconciles sent %d requests, want one", calls)
	}
	now = now.Add(2 * time.Minute)
	_, _ = c.Select(context.Background(), flyingAbility)
	if calls != 2 {
		t.Fatalf("request did not resume after cooldown: %d", calls)
	}
}

func TestBackoffIncreasesUntilSuccess(t *testing.T) {
	now := time.Now()
	c := NewClient(testAPIKey, "")
	c.now = func() time.Time { return now }
	for _, minutes := range []int{1, 2, 4, 8, 15, 15} {
		var limited *RateLimitError
		if !errors.As(c.rateLimited("", nil), &limited) || limited.RetryAt.Sub(now) != time.Duration(minutes)*time.Minute {
			t.Fatalf("expected %d minute backoff, got %v", minutes, limited)
		}
		now = limited.RetryAt
	}
	c.rememberDecision(sha256.Sum256([]byte("success")), []Match{{Emoji: "🦅", Name: eagleName, Score: 1}})
	var limited *RateLimitError
	if !errors.As(c.rateLimited("", nil), &limited) || limited.RetryAt.Sub(now) != time.Minute {
		t.Fatalf("success did not reset backoff: %v", limited)
	}
}

func TestSelectionResumesAfterRateLimit(t *testing.T) {
	requests := make(map[[sha256.Size]byte]int)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		calls++
		requests[sha256.Sum256(body)]++
		if calls == 2 {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var req struct {
			Questions map[string]question `json:"questions"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Error(err)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{"answers": stubAnswers(req.Questions)},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	now := time.Now()
	c := NewClient(testAPIKey, "")
	c.endpoint, c.now = server.URL, func() time.Time { return now }
	if _, err := c.Select(context.Background(), flyingAbility); err == nil {
		t.Fatal("expected the second batch to be rate limited")
	}
	now = now.Add(time.Minute)
	selected, err := c.Select(context.Background(), flyingAbility)
	if err != nil || len(selected) != 1 || selected[0].Emoji != "🦅" {
		t.Fatalf("selection did not recover: %v, %v", selected, err)
	}
	repeated := 0
	for _, count := range requests {
		if count == 2 {
			repeated++
		} else if count != 1 {
			t.Fatalf("unexpected request count: %d", count)
		}
	}
	if repeated != 1 {
		t.Fatalf("only the throttled batch should be repeated, got %d", repeated)
	}
	before := calls
	if _, err := c.Select(context.Background(), flyingAbility); err != nil || calls != before {
		t.Fatalf("successful decisions should be reused: calls=%d, want=%d, err=%v", calls, before, err)
	}
}

func TestDecisionCacheIsBoundedAndExpires(t *testing.T) {
	now := time.Now()
	c := NewClient(testAPIKey, "")
	c.now = func() time.Time { return now }
	match := []Match{{Emoji: "🦅", Name: eagleName, Score: 1}}
	for i := range maxCachedDecisions + 1 {
		c.rememberDecision(sha256.Sum256(fmt.Appendf(nil, "ability-%d", i)), match)
		now = now.Add(time.Second)
	}
	if len(c.decisions) != maxCachedDecisions {
		t.Fatalf("unbounded cache: %d", len(c.decisions))
	}
	first, _ := c.lookupDecision(sha256.Sum256([]byte("ability-0")))
	if first != nil {
		t.Fatal("oldest decision was not evicted")
	}
	key := sha256.Sum256([]byte("ability-1"))
	before, _ := c.lookupDecision(key)
	if len(before) != 1 {
		t.Fatal("recent decision was not cached")
	}
	before[0].Emoji = "changed"
	again, _ := c.lookupDecision(key)
	if again[0].Emoji != "🦅" {
		t.Fatal("callers must not mutate the cache")
	}
	now = now.Add(decisionLifetime)
	after, _ := c.lookupDecision(key)
	if after != nil {
		t.Fatal("expired decision was reused")
	}
}
