// Package emoji matches super abilities to Unicode emoji through Jev.
package emoji

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"strings"
)

// Unicode's emoji test data is bundled so reconciliation never downloads a catalog.
// See UNICODE-LICENSE.txt and https://unicode.org/Public/emoji/latest/emoji-test.txt.
//
//go:embed emoji-test.txt
var catalogText string

var catalog = parseCatalog(catalogText)

func parseCatalog(data string) map[string]string {
	entries := make(map[string]string)
	for line := range strings.SplitSeq(data, "\n") {
		definition, description, ok := strings.Cut(line, "#")
		if !ok || !strings.Contains(definition, "; fully-qualified") {
			continue
		}
		fields := strings.Fields(description)
		if len(fields) >= 3 {
			entries[fields[0]] = strings.Join(fields[2:], " ")
		}
	}
	return entries
}

// Match is a scored emoji from the final comparison, not from an individual batch.
type Match struct {
	Emoji string  `json:"emoji"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// ValidMatches checks persisted results before reusing them.
func ValidMatches(matches []Match) bool {
	if len(matches) == 0 || len(matches) > 3 {
		return false
	}
	seen := make(map[string]bool)
	for _, m := range matches {
		if name, ok := catalog[m.Emoji]; !ok || name != m.Name || seen[m.Emoji] || !validProbability(m.Score) {
			return false
		}
		seen[m.Emoji] = true
	}
	return true
}

func catalogHash() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(catalogText)))
}
