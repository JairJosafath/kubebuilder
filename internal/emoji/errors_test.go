package emoji

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestThrottlingAndOverloadStartCooldown(t *testing.T) {
	for _, test := range []struct {
		status int
		cause  string
	}{
		{http.StatusTooManyRequests, "rate limit exceeded"},
		{statusOverloaded, "service overloaded"},
	} {
		t.Run(fmt.Sprint(test.status), func(t *testing.T) {
			c := NewClient(testAPIKey, "")
			now := time.Now()
			c.now = func() time.Time { return now }
			err := c.responseError(&http.Response{
				StatusCode: test.status,
				Header:     http.Header{"Retry-After": []string{"120"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"test-key"}`)),
			})
			var limited *RateLimitError
			if !errors.As(err, &limited) || !limited.RetryAt.Equal(now.Add(2*time.Minute)) || limited.Status != test.status {
				t.Fatalf("expected a cooldown until Retry-After: %v", err)
			}
			if !strings.Contains(err.Error(), test.cause) || strings.Contains(err.Error(), testAPIKey) {
				t.Fatalf("unexpected or unsafe diagnostic: %v", err)
			}
			_, cooldown := c.lookupDecision([32]byte{})
			if cooldown == nil || cooldown.Error() != err.Error() {
				t.Fatalf("cooldown lost original diagnostic: %v", cooldown)
			}
		})
	}
}

func TestOtherHTTPErrorsOmitResponseBody(t *testing.T) {
	c := NewClient(testAPIKey, "")
	err := c.responseError(&http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"error":"invalid key test-key"}`)),
	})
	if err.Error() != "jev returned HTTP 401" {
		t.Fatalf("unexpected error: %v", err)
	}
}
