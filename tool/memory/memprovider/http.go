package memprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultTimeout caps any single HTTP call to the remote memory service.
const defaultTimeout = 20 * time.Second

// httpClient is the package-level client reused by all providers so
// connection pooling works across calls.
var httpClient = &http.Client{Timeout: defaultTimeout}

// httpJSON sends body as JSON to the given URL with method and
// optional Bearer auth. The response JSON is decoded into out (may
// be nil to discard). Non-2xx responses return an error containing
// the status code and (truncated) body for debugging.
func httpJSON(ctx context.Context, method, url, bearer string, body, out any) error {
	return httpJSONWith(ctx, httpClient, method, url, bearer, body, out)
}

func httpJSONWith(ctx context.Context, client *http.Client, method, url, bearer string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("memprovider: encode body: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return fmt.Errorf("memprovider: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("memprovider: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("memprovider: %s %s: status %d: %s",
			method, url, resp.StatusCode, string(errBody))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("memprovider: decode response: %w", err)
	}
	return nil
}
