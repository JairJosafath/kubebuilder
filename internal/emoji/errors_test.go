package emoji

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestResponseErrorKeepsOnlyNumericCode(t *testing.T) {
	for _, test := range []struct {
		name     string
		body     string
		wantCode bool
	}{
		{"community throttle", `{"code":-1,"message":"Too many requests. Please try again later."}`, true},
		{"credential echo", `{"code":-1,"message":"test-key","data":{"key":"test-key"}}`, true},
		{"non-JSON", testAPIKey, false},
		{"non-numeric code", `{"code":"test-key"}`, false},
		{"oversized", `{"code":-1,"message":"` + strings.Repeat("x", 8*1024) + `"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := NewClient(testAPIKey, "")
			now := time.Now()
			c.now = func() time.Time { return now }
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": []string{"120"}},
				Body:       io.NopCloser(strings.NewReader(test.body)),
			}
			err := c.responseError(resp)
			var limited *RateLimitError
			if !errors.As(err, &limited) || !limited.RetryAt.Equal(now.Add(2*time.Minute)) {
				t.Fatalf("expected rate-limit deadline: %v", err)
			}
			if (limited.APICode != nil) != test.wantCode || (test.wantCode && *limited.APICode != -1) {
				t.Fatalf("unexpected API code: %v", limited.APICode)
			}
			if strings.Contains(err.Error(), testAPIKey) || strings.Contains(err.Error(), "account limits") {
				t.Fatalf("unsafe or unsupported diagnostic: %v", err)
			}
			_, cooldown := c.lookupDecision([32]byte{})
			if cooldown == nil || cooldown.Error() != err.Error() {
				t.Fatalf("cooldown lost original diagnostic: %v", cooldown)
			}
		})
	}
}

func TestOtherHTTPErrorKeepsNumericCode(t *testing.T) {
	c := NewClient(testAPIKey, "")
	err := c.responseError(&http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"code":-2,"message":"test-key"}`)),
	})
	if err.Error() != "jev returned HTTP 401 (API code -2)" {
		t.Fatalf("unexpected error: %v", err)
	}
}
