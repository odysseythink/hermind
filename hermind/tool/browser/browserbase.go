package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/odysseythink/hermind/config"
)

// BrowserbaseProvider implements Provider against the Browserbase API.
type BrowserbaseProvider struct {
	cfg    config.BrowserbaseConfig
	client *http.Client
}

// NewBrowserbase builds a Browserbase provider from config, merging in
// environment variables if set.
func NewBrowserbase(cfg config.BrowserbaseConfig) *BrowserbaseProvider {
	if v := os.Getenv("BROWSERBASE_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("BROWSERBASE_PROJECT_ID"); v != "" {
		cfg.ProjectID = v
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.browserbase.com"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &BrowserbaseProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *BrowserbaseProvider) Name() string { return "browserbase" }

func (b *BrowserbaseProvider) IsConfigured() bool {
	return b.cfg.APIKey != "" && b.cfg.ProjectID != ""
}

// do sends a JSON body to the Browserbase API using the X-BB-API-Key
// auth header (Browserbase does not use Bearer auth).
func (b *BrowserbaseProvider) do(ctx context.Context, method, path string, body any, out any) error {
	if !b.IsConfigured() {
		return errors.New("browserbase: missing BROWSERBASE_API_KEY or BROWSERBASE_PROJECT_ID")
	}
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("browserbase: encode: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.cfg.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-BB-API-Key", b.cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("browserbase: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("browserbase: %s %s: status %d: %s",
			method, path, resp.StatusCode, string(bodyBytes))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type bbCreateResponse struct {
	ID         string `json:"id"`
	ConnectURL string `json:"connectUrl"`
}

// CreateSession calls POST /v1/sessions and returns the new session.
func (b *BrowserbaseProvider) CreateSession(ctx context.Context) (*Session, error) {
	body := map[string]any{"projectId": b.cfg.ProjectID}
	if b.cfg.KeepAlive {
		body["keepAlive"] = true
	}
	if b.cfg.Proxies {
		body["proxies"] = true
	}
	var resp bbCreateResponse
	if err := b.do(ctx, "POST", "/v1/sessions", body, &resp); err != nil {
		return nil, err
	}
	sess := &Session{
		ID:         resp.ID,
		ConnectURL: resp.ConnectURL,
		Provider:   b.Name(),
	}
	if live, err := b.LiveURL(ctx, resp.ID); err == nil {
		sess.LiveURL = live
	}
	return sess, nil
}

type bbDebugResponse struct {
	DebuggerFullscreenURL string `json:"debuggerFullscreenUrl"`
	DebuggerURL           string `json:"debuggerUrl"`
}

func (b *BrowserbaseProvider) LiveURL(ctx context.Context, id string) (string, error) {
	var resp bbDebugResponse
	if err := b.do(ctx, "GET", "/v1/sessions/"+id+"/debug", nil, &resp); err != nil {
		return "", err
	}
	if resp.DebuggerFullscreenURL != "" {
		return resp.DebuggerFullscreenURL, nil
	}
	return resp.DebuggerURL, nil
}

func (b *BrowserbaseProvider) CloseSession(ctx context.Context, id string) error {
	body := map[string]any{
		"projectId": b.cfg.ProjectID,
		"status":    "REQUEST_RELEASE",
	}
	return b.do(ctx, "POST", "/v1/sessions/"+id, body, nil)
}
