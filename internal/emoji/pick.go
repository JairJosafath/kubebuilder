package emoji

const (
	// closeScoreGap is how far below the winner a match may score and still be shown.
	closeScoreGap = 0.05
	// scoreTolerance absorbs floating-point error when comparing score gaps.
	scoreTolerance = 1e-9
	// maxShown is the most emoji the page shows, as a 2×2 square.
	maxShown = 4
)

// Pick returns the matches the webpage shows from ranked, which must be sorted
// best first: a clear winner alone, two close matches side by side, or the top
// four in a 2×2 square. When three are close, the fourth completes the square.
func Pick(ranked []Match) []Match {
	if len(ranked) == 0 {
		return nil
	}
	n := 1
	for n < min(maxShown, len(ranked)) && ranked[0].Score-ranked[n].Score <= closeScoreGap+scoreTolerance {
		n++
	}
	if n == 3 {
		n = 2
		if len(ranked) >= maxShown {
			n = maxShown
		}
	}
	return ranked[:n]
}
