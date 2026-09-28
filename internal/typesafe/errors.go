package typesafe

import (
	"fmt"
	"net/http"
)

// statusOverloaded is TypeSafe's "529 Overloaded"; net/http has no name for it.
const statusOverloaded = 529

// responseError reports only the HTTP status. Provider error text is never
// copied into status or logs because it can echo credentials or request contents.
func (c *Client) responseError(resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == statusOverloaded {
		return c.rateLimited(resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	return fmt.Errorf("jev returned HTTP %d", resp.StatusCode)
}
