package emoji

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestPackingRespectsQuestionAndByteLimits(t *testing.T) {
	c := NewClient(testAPIKey, "")
	small := make(map[string]question)
	for i := range maxQuestions + 1 {
		small[fmt.Sprintf("q%d", i)] = emojiQuestion(map[string]string{"a": "first", "b": "second"})
	}
	packed, err := c.packQuestions(flyingAbility, small)
	if err != nil || len(packed) != 2 || len(packed[0]) != maxQuestions || len(packed[1]) != 1 {
		t.Fatalf("question limit not respected: %v, %v", packed, err)
	}
	// The CRD permits 256 Unicode characters; use a multi-byte ability to exercise
	// the byte budget as well as the Choice and question-count limits.
	ability := strings.Repeat("🦅", 256)
	questions, err := c.selectionQuestions(ability)
	if err != nil {
		t.Fatal(err)
	}
	packed, err = c.packQuestions(ability, questions)
	if err != nil {
		t.Fatal(err)
	}
	again, err := c.packQuestions(ability, questions)
	if err != nil || len(again) != len(packed) {
		t.Fatalf("non-deterministic packing: %v", err)
	}
	seen := make(map[string]bool)
	for i, request := range packed {
		body, err := c.decisionBody(ability, request)
		repeated, repeatErr := c.decisionBody(ability, again[i])
		if err != nil || repeatErr != nil || len(body) > maxRequestBytes || len(request) > maxQuestions || !bytes.Equal(body, repeated) {
			t.Fatalf("invalid packed request %d: bytes=%d questions=%d", i, len(body), len(request))
		}
		for _, q := range request {
			if len(q.Criteria) < 2 || len(q.Criteria) > maxChoiceOptions {
				t.Fatalf("invalid Choice size: %d", len(q.Criteria))
			}
			for symbol := range q.Criteria {
				if seen[symbol] {
					t.Fatalf("duplicate catalog entry: %s", symbol)
				}
				seen[symbol] = true
			}
		}
	}
	if len(seen) != len(catalog) {
		t.Fatalf("catalog coverage: %d/%d", len(seen), len(catalog))
	}
}
