package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/config"
)

type CamofoxProvider struct {
	cfg    config.CamofoxConfig
	client *http.Client
}

func NewCamofox(cfg config.CamofoxConfig) *CamofoxProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:9377"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &CamofoxProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CamofoxProvider) Name() string { return "camofox" }

func (c *CamofoxProvider) IsConfigured() bool {
	return c.cfg.BaseURL != ""
}

type camofoxCreateResponse struct {
	ID     string `json:"id"`
	CDPURL string `json:"cdp_url"`
	VNCURL string `json:"vnc_url"`
}

func (c *CamofoxProvider) CreateSession(ctx context.Context) (*Session, error) {
	body := map[string]any{}
	if c.cfg.ManagedPersistence {
		body["persist"] = true
	}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/sessions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("camofox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("camofox: create session: status %d: %s", resp.StatusCode, string(respBody))
	}
	var cr camofoxCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("camofox: decode: %w", err)
	}
	return &Session{
		ID:         cr.ID,
		ConnectURL: cr.CDPURL,
		LiveURL:    cr.VNCURL,
		Provider:   c.Name(),
	}, nil
}

func (c *CamofoxProvider) CloseSession(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.cfg.BaseURL+"/sessions/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("camofox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("camofox: close session: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

type camofoxSessionResponse struct {
	VNCURL string `json:"vnc_url"`
}

func (c *CamofoxProvider) LiveURL(ctx context.Context, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/sessions/"+id, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("camofox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("camofox: get session: status %d: %s", resp.StatusCode, string(body))
	}
	var sr camofoxSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("camofox: decode: %w", err)
	}
	return sr.VNCURL, nil
}
