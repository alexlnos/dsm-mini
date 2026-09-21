package dsm

import (
	"context"
	"fmt"
	"net/http"
)

// HTTPError means DSM answered with a status other than 200.
//
// A separate type is needed because a broken API handler returns 502, and
// that is the only sign by which we can decide to lower the call version.
type HTTPError struct {
	Status int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("DSM answered HTTP %d", e.Status)
}

// retryLowerVersion reports whether repeating the call at a lower version makes sense.
//
// There are two reasons:
//
//   - HTTP 5xx: on some builds the handler of a particular API version crashes.
//     Seen on File Station 1.4.4-2221, where SYNO.FileStation.List version 2
//     answers 502 for any parameters while version 1 works.
//   - Codes 103 and 104: this NAS has no such method or version.
func retryLowerVersion(err error) bool {
	var httpErr *HTTPError
	if asHTTPError(err, &httpErr) {
		return httpErr.Status >= http.StatusInternalServerError
	}
	var apiErr *APIError
	if asAPIError(err, &apiErr) {
		return apiErr.Code == 103 || apiErr.Code == 104
	}
	return false
}

// CallVersioned invokes a method, walking versions from the first to the last
// until one works. It remembers the version that worked and starts with it
// next time.
//
// The order of versions is up to the caller: usually newest to oldest.
func (c *Client) CallVersioned(ctx context.Context, api, method string, versions []int,
	params map[string]any, out any) error {

	if len(versions) == 0 {
		return fmt.Errorf("%s.%s: no versions given", api, method)
	}

	key := api + "." + method
	c.mu.RLock()
	known, ok := c.goodVersion[key]
	c.mu.RUnlock()
	if ok {
		versions = append([]int{known}, without(versions, known)...)
	}

	var lastErr error
	for _, v := range versions {
		err := c.Call(ctx, api, method, v, params, out)
		if err == nil {
			c.mu.Lock()
			c.goodVersion[key] = v
			c.mu.Unlock()
			return nil
		}
		lastErr = err
		if !retryLowerVersion(err) {
			return err
		}
		c.log.Debug("the API version did not work, trying a lower one",
			"api", api, "method", method, "version", v, "err", err)
	}
	return lastErr
}

func without(xs []int, v int) []int {
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func asHTTPError(err error, target **HTTPError) bool {
	for err != nil {
		if e, ok := err.(*HTTPError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
