package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/odysseythink/hermind/provider"
)

// ListModels queries GET {BaseURL}/v1/models with Anthropic's header
// shape (x-api-key, anthropic-version). Parses the OpenAI-compatible
// `{"data":[{"id":...}]}` response. Satisfies provider.ModelLister.
func (a *Anthropic) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("anthropic: list models request: %w", err)
	}
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", defaultAPIVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "anthropic",
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mapHTTPError(resp)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("anthropic: decode list models: %w", err)
	}

	ids := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// Compile-time check: *Anthropic satisfies provider.ModelLister.
var _ provider.ModelLister = (*Anthropic)(nil)
