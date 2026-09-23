package emoji

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

const (
	eagleName  = "eagle"
	testAPIKey = "test-key"
)

func TestRankingAndCloseScores(t *testing.T) {
	options := map[string]string{"🦅": eagleName, "🪽": "wing", "✈️": "airplane", "🚀": "rocket"}
	for _, test := range []struct {
		name   string
		scores map[string]float64
		want   int
	}{
		{"clear winner", map[string]float64{"🦅": .8, "🪽": .1, "✈️": .06, "🚀": .04}, 1},
		{"two close", map[string]float64{"🦅": .4, "🪽": .35, "✈️": .2, "🚀": .05}, 2},
		{"four tied capped at three", map[string]float64{"🦅": .25, "🪽": .25, "✈️": .25, "🚀": .25}, 3},
		{"compare to winner not neighbor", map[string]float64{"🦅": .36, "🪽": .32, "✈️": .28, "🚀": .04}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			ranked, err := rank(options, test.scores)
			if err != nil {
				t.Fatal(err)
			}
			selected := closeMatches(ranked)
			if len(selected) != test.want || !ValidMatches(selected) {
				t.Fatalf("unexpected matches: %+v", selected)
			}
			for i := 1; i < len(ranked); i++ {
				if ranked[i].Score > ranked[i-1].Score ||
					(ranked[i].Score == ranked[i-1].Score && ranked[i].Emoji < ranked[i-1].Emoji) {
					t.Fatal("ranking must be descending with deterministic ties")
				}
			}
		})
	}
}

func TestRejectInvalidDistribution(t *testing.T) {
	options := map[string]string{"🦅": eagleName, "🪽": "wing"}
	for _, scores := range []map[string]float64{
		nil, {"🦅": 1}, {"🦅": .5, "unknown": .5}, {"🦅": -.1, "🪽": 1.1},
		{"🦅": 0, "🪽": 0}, {"🦅": math.NaN(), "🪽": .5}, {"🦅": math.Inf(1), "🪽": .5},
	} {
		if _, err := rank(options, scores); err == nil {
			t.Fatalf("accepted invalid distribution: %v", scores)
		}
	}
}

func TestSelectUsesEntireCatalogAndReranksFinalists(t *testing.T) {
	if len(catalog) < 3900 || catalog["🦅"] != eagleName || catalog["👩🏽‍🚀"] == "" || catalog["🇲🇽"] == "" {
		t.Fatal("catalog must include Unicode sequences, modifiers, and flags")
	}
	seen := make(map[string]bool)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-key" ||
			r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect Jev request headers")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) > 32*1024 {
			t.Error("invalid request size")
		}
		var req struct {
			State     map[string]string   `json:"state"`
			Questions map[string]question `json:"questions"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Error(err)
		}
		q := req.Questions["emoji"]
		if len(q.Criteria) > 255 || len(q.Criteria) < 2 || q.Type != "choice" || req.State["super_ability"] != "Flying" {
			t.Error("incorrect Jev question")
		}
		probabilities := make(map[string]float64)
		for symbol := range q.Criteria {
			seen[symbol] = true
			probabilities[symbol] = 0
		}
		if _, ok := q.Criteria["🦅"]; ok {
			probabilities["🦅"] = 1
		} else {
			probabilities[slices.Sorted(maps.Keys(q.Criteria))[0]] = 1
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{"answers": map[string]any{"emoji": map[string]any{
				"type": "choice", "probabilities": probabilities,
			}}},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c := NewClient(testAPIKey, "")
	c.endpoint = server.URL
	selected, err := c.Select(context.Background(), "Flying")
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Emoji != "🦅" || len(seen) != len(catalog) || calls < 2 {
		t.Fatalf("incomplete selection: matches=%v catalog=%d/%d calls=%d", selected, len(seen), len(catalog), calls)
	}
}

func TestJevErrorsAreSafe(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
		body string
	}{
		{"unauthorized", 401, testAPIKey}, {"rate limit", 429, testAPIKey},
		{"malformed", 200, testAPIKey}, {"service error", 200, `{"code":1,"message":"test-key"}`},
		{"missing envelope", 200, `{"data":{"answers":{}}}`},
		{"missing answer", 200, `{"code":0,"data":{"answers":{}}}`},
		{"oversized", 200, strings.Repeat("x", 128*1024+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.code)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			c := NewClient(testAPIKey, "")
			c.endpoint = server.URL
			_, err := c.Select(context.Background(), "Flying")
			if err == nil || strings.Contains(err.Error(), testAPIKey) {
				t.Fatalf("expected sanitized error: %v", err)
			}
		})
	}
	if _, err := NewClient("", "").Select(context.Background(), "Flying"); err == nil {
		t.Fatal("missing credentials must fail without an API call")
	}
}

func TestCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewClient(testAPIKey, "").Select(ctx, "Flying"); err == nil {
		t.Fatal("canceled request must fail")
	}
}
