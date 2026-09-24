package emoji

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// responseError keeps the community API's numeric error code without copying
// untrusted provider text (which can echo credentials or request contents).
func (c *Client) responseError(resp *http.Response) error {
	var envelope struct {
		Code *int `json:"code"`
	}
	const maxErrorBytes = 8 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes+1))
	if err != nil || len(body) > maxErrorBytes || json.Unmarshal(body, &envelope) != nil {
		envelope.Code = nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return c.rateLimited(resp.Header.Get("Retry-After"), envelope.Code)
	}
	if envelope.Code != nil {
		return fmt.Errorf("jev returned HTTP %d (API code %d)", resp.StatusCode, *envelope.Code)
	}
	return fmt.Errorf("jev returned HTTP %d", resp.StatusCode)
}
