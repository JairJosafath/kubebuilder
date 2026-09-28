// Package emoji is the provider-neutral emoji vocabulary: the bundled Unicode
// catalog, scored matches, and the rule that decides how many matches the
// webpage shows. Providers such as internal/typesafe produce Matches; this
// package never contacts them.
package emoji

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"maps"
	"math"
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

// Catalog returns every fully-qualified emoji in the bundled Unicode data,
// mapped to its name. The caller owns the returned map.
func Catalog() map[string]string {
	return maps.Clone(catalog)
}

// CatalogVersion identifies the bundled catalog. Providers include it in their
// cache keys, so updating the catalog invalidates stored selections.
func CatalogVersion() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(catalogText)))
}

// Match is a scored emoji from the final comparison, not from an individual batch.
type Match struct {
	Emoji string  `json:"emoji"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// ValidScore reports whether score is a finite probability between 0 and 1.
func ValidScore(score float64) bool {
	return !math.IsNaN(score) && !math.IsInf(score, 0) && score >= 0 && score <= 1
}

// ValidMatches checks persisted results before reusing them. The page shows
// one, two, or four emoji.
func ValidMatches(matches []Match) bool {
	if n := len(matches); n != 1 && n != 2 && n != 4 {
		return false
	}
	seen := make(map[string]bool)
	for _, m := range matches {
		if name, ok := catalog[m.Emoji]; !ok || name != m.Name || seen[m.Emoji] || !ValidScore(m.Score) {
			return false
		}
		seen[m.Emoji] = true
	}
	return true
}
