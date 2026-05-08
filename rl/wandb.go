package rl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type WandBRunStatus struct {
	State   string             `json:"state"`
	Config  map[string]any     `json:"config"`
	Summary map[string]float64 `json:"summary"`
}

type WandBClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewWandBClient(apiKey, baseURL string) *WandBClient {
	if baseURL == "" {
		baseURL = "https://api.wandb.ai"
	}
	return &WandBClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (w *WandBClient) GetRunStatus(ctx context.Context, path string) (*WandBRunStatus, error) {
	if w.apiKey == "" {
		return nil, fmt.Errorf("wandb: api key not configured")
	}
	url := w.baseURL + "/api/v1/" + path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.apiKey)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wandb: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("wandb: status %d: %s", resp.StatusCode, string(body))
	}
	var status WandBRunStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("wandb: decode: %w", err)
	}
	return &status, nil
}
