package platforms

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// httpProbe sends an HTTP request and returns nil on 2xx status, or a
// user-facing error with a short body excerpt otherwise. Headers may be
// nil. Uses http.DefaultClient; the caller's ctx controls cancellation
// and deadline.
//
// Descriptor Test closures wrap this helper with platform-specific
// URL + header assembly. The /api/platforms/{key}/test handler surfaces
// the returned error.Error() verbatim as the `error` field of the JSON
// response body.
func httpProbe(ctx context.Context, method, url string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	snippet := strings.TrimSpace(string(body))
	if snippet == "" {
		return fmt.Errorf("probe failed: status %d", resp.StatusCode)
	}
	return fmt.Errorf("probe failed: status %d: %s", resp.StatusCode, snippet)
}
