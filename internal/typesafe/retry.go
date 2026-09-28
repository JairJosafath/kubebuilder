package typesafe

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jairjosafath/operator/internal/emoji"
)

const (
	maxCachedDecisions = 128
	decisionLifetime   = time.Hour
	maxBackoff         = 15 * time.Minute
)

type cachedDecision struct {
	matches []emoji.Match
	expires time.Time
}

// Successful batches survive retries in this process, so a throttled scan can
// resume without paying for completed batches again. The cache is bounded.
func (c *Client) lookupDecision(key [sha256.Size]byte) ([]emoji.Match, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if cached, ok := c.decisions[key]; ok {
		if now.Before(cached.expires) {
			return slices.Clone(cached.matches), nil
		}
		delete(c.decisions, key)
	}
	// Apply the cooldown across abilities and Superpods sharing this client.
	// Kubernetes watch events cannot bypass it by reconciling early.
	if now.Before(c.retryAt) {
		return nil, &emoji.RateLimitError{RetryAt: c.retryAt, Cause: rateLimitCause(c.retryStatus)}
	}
	return nil, nil
}

func (c *Client) rememberDecision(key [sha256.Size]byte, matches []emoji.Match) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.decisions == nil {
		c.decisions = make(map[[sha256.Size]byte]cachedDecision)
	}
	if len(c.decisions) >= maxCachedDecisions {
		var oldest [sha256.Size]byte
		var expires time.Time
		for entry, cached := range c.decisions {
			if expires.IsZero() || cached.expires.Before(expires) {
				oldest, expires = entry, cached.expires
			}
		}
		delete(c.decisions, oldest)
	}
	c.decisions[key] = cachedDecision{matches: slices.Clone(matches), expires: c.now().Add(decisionLifetime)}
	c.backoff = 0
}

func (c *Client) rateLimited(status int, retryAfter string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backoff = min(max(time.Minute, c.backoff*2), maxBackoff)
	now := c.now()
	deadline := retryDeadline(retryAfter, now, c.backoff)
	if deadline.After(c.retryAt) {
		c.retryAt = deadline
	}
	c.retryStatus = status
	return &emoji.RateLimitError{RetryAt: c.retryAt, Cause: rateLimitCause(c.retryStatus)}
}

// rateLimitCause describes a throttled (429) or overloaded (529) response. It
// deliberately excludes the provider response body and credentials.
func rateLimitCause(status int) string {
	cause := "rate limit exceeded"
	if status == statusOverloaded {
		cause = "service overloaded"
	}
	return fmt.Sprintf("Jev returned HTTP %d (%s)", status, cause)
}

func retryDeadline(header string, now time.Time, fallback time.Duration) time.Time {
	header = strings.TrimSpace(header)
	// Bound multiplication to avoid overflowing time.Duration on invalid input.
	if seconds, err := strconv.ParseUint(header, 10, 63); err == nil && seconds > 0 &&
		seconds <= uint64((1<<63-1)/int64(time.Second)) {
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if deadline, err := http.ParseTime(header); err == nil && deadline.After(now) {
		return deadline
	}
	return now.Add(fallback)
}
