package emoji

import (
	"cmp"
	"fmt"
	"time"
)

// RateLimitError reports that the emoji provider refused a request and when the
// next one may be sent. Its message is copied into Superpod status, so Cause
// must never contain provider response text or credentials.
type RateLimitError struct {
	RetryAt time.Time
	// Cause describes the refusal, such as "Jev returned HTTP 429 (rate limit exceeded)".
	Cause string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%s; next attempt after %s", cmp.Or(e.Cause, "emoji provider rate limit exceeded"),
		e.RetryAt.UTC().Format(time.RFC3339))
}
