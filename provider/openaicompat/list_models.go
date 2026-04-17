package openaicompat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/odysseythink/hermind/provider"
)

// ListModels queries GET {BaseURL}/models with the configured Bearer
// auth and parses the OpenAI-standard `{"data":[{"id":...}]}` shape.
// Returns model IDs in the order the server returned them. Satisfies
// provider.ModelLister.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("%s: list models request: %w", c.cfg.ProviderName, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	for k, v := range c.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: c.cfg.ProviderName,
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mapHTTPError(c.cfg.ProviderName, resp)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("%s: decode list models: %w", c.cfg.ProviderName, err)
	}

	ids := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// Compile-time check: *Client satisfies provider.ModelLister.
var _ provider.ModelLister = (*Client)(nil)
